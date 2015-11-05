package sign

import (
	"bytes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/logutils"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
)

type Type int // used by other modules as coll_sign.Type

const (
	// Default Signature involves creating Merkle Trees
	MerkleTree = iota
	// Basic Signature removes all Merkle Trees
	// Collective public keys are still created and can be used
	PubKey
	// Basic Signature on aggregated votes
	Voter
)

var _ Signer = &Node{}

type Node struct {
	coconet.Host

	// Signing Node will Fail at FailureRate probability
	FailureRate         int
	FailAsRootEvery     int
	FailAsFollowerEvery int

	randmu sync.Mutex
	Rand   *rand.Rand

	Type   Type
	Height int

	HostList []string

	suite   abstract.Suite
	PubKey  abstract.Point  // long lasting public key
	PrivKey abstract.Secret // long lasting private key

	nRounds       int
	Rounds        map[int]*Round
	Round         int // *only* used by Root( by annoucer)
	RoundTypes    []RoundType
	roundmu       sync.Mutex
	LastSeenRound int // largest round number I have seen
	RoundsAsRoot  int // latest continuous streak of rounds with sn root

	callbacks Callbacks
	AnnounceLock sync.Mutex

	// NOTE: reuse of channels via round-number % Max-Rounds-In-Mermory can be used
	roundLock sync.RWMutex
	Message   []byte                    // for testing purposes
	peerKeys  map[string]abstract.Point // map of all peer public keys

	closed      chan error // error sent when connection closed
	Isclosed    bool
	done        chan int // round number sent when round done
	commitsDone chan int // round number sent when announce/commit phase done

	RoundsPerView int
	// "root" or "regular" are sent on this channel to
	// notify the maker of the sn what role sn plays in the new view
	viewChangeCh chan string
	ChangingView bool // TRUE if node is currently engaged in changing the view
	viewmu       sync.Mutex
	ViewNo       int

	timeout  time.Duration
	timeLock sync.RWMutex

	hbLock    sync.Mutex
	heartbeat *time.Timer

	// ActionsLock sync.Mutex
	// Actions     []*VoteRequest

	VoteLog         *VoteLog // log of all confirmed votes, useful for replay
	LastSeenVote    int64    // max of all Highest Votes we've seen, and our last commited vote
	LastAppliedVote int64    // last vote we have committed to our log

	Actions map[int][]*Vote

	// These are stored during the challenge phase so that they can
	// be sent to the client during the SignatureBroadcast
	Proof  proof.Proof
	MTRoot hashid.HashId // the very root of the big Merkle Tree
}

// Set callback-functions for the different steps of the algorithm
func (sn *Node)SetCallbacks(cb Callbacks){
	sn.callbacks = cb
}

// Start listening for messages coming from parent(up)
func (sn *Node) Listen() error {
	if sn.Pool() == nil {
		sn.GenSetPool()
	}
	err := sn.getMessages()
	return err
}

// func (sn *Node) CheckRoundTypes(rts []RoundType) error {
// 	if len(rts) != len(sn.RoundTypes)
// 	for i := range sn.RoundTypes {
//
//
// 	}
// }
//
func (sn *Node) printRoundTypes() {
	sn.roundmu.Lock()
	defer sn.roundmu.Unlock()
	for i, rt := range sn.RoundTypes {
		if i > sn.LastSeenRound {
			break
		}
		log.Println("Round", i, "type", rt.String())
	}
}

func (sn *Node) Close() {
	// sn.printRoundTypes()
	sn.hbLock.Lock()
	if sn.heartbeat != nil {
		sn.heartbeat.Stop()
		sn.heartbeat = nil
		dbg.Lvl4("after close", sn.Name(), "has heartbeat=", sn.heartbeat)
	}
	if !sn.Isclosed {
		close(sn.closed)
		dbg.Lvl4("signing node: closing:", sn.Name())
		sn.Host.Close()
	}
	dbg.Lvl3("Closed connection")
	sn.Isclosed = true
	sn.hbLock.Unlock()
}

func (sn *Node) ViewChangeCh() chan string {
	return sn.viewChangeCh
}

func (sn *Node) Hostlist() []string {
	return sn.HostList
}

// Returns name of node who should be the root for the next view
// round robin is used on the array of host names to determine the next root
func (sn *Node) RootFor(view int) string {
	// log.Println(sn.Name(), "ROOT FOR", view)
	var hl []string
	if view == 0 {
		hl = sn.HostListOn(view)
	} else {
		// we might not have the host list for current view
		// safer to use the previous view's hostlist, always
		hl = sn.HostListOn(view - 1)
	}
	return hl[view%len(hl)]
}

func (sn *Node) SetFailureRate(v int) {
	sn.FailureRate = v
}

func (sn *Node) logFirstPhase(firstRoundTime time.Duration) {
	log.WithFields(log.Fields{
		"file":  logutils.File(),
		"type":  "root_announce",
		"round": sn.nRounds,
		"time":  firstRoundTime,
	}).Info("done with root announce round " + strconv.Itoa(sn.nRounds))
}

func (sn *Node) logSecondPhase(secondRoundTime time.Duration) {
	log.WithFields(log.Fields{
		"file":  logutils.File(),
		"type":  "root_challenge second",
		"round": sn.nRounds,
		"time":  secondRoundTime,
	}).Info("done with root challenge round " + strconv.Itoa(sn.nRounds))
}

func (sn *Node) logTotalTime(totalTime time.Duration) {
	log.WithFields(log.Fields{
		"file":  logutils.File(),
		"type":  "root_challenge total",
		"round": sn.nRounds,
		"time":  totalTime,
	}).Info("done with root challenge round " + strconv.Itoa(sn.nRounds))
}

var MAX_WILLING_TO_WAIT time.Duration = 50 * time.Second

var ChangingViewError error = errors.New("In the process of changing view")

func (sn *Node) StartAnnouncement(am *AnnouncementMessage) error {
	sn.AnnounceLock.Lock()
	defer sn.AnnounceLock.Unlock()

	// notify upstream of announcement
	if sn.callbacks != nil {
		sn.callbacks.Announcement(am)
	}

	dbg.Lvl2("root", sn.Name(), "starting announcement round for round: ", sn.nRounds, "on view", sn.ViewNo)

	/*
		first := time.Now()
		total := time.Now()
		var firstRoundTime time.Duration
		var totalTime time.Duration
	*/

	ctx, cancel := context.WithTimeout(context.Background(), MAX_WILLING_TO_WAIT)
	var cancelederr error
	go func() {
		var err error
		if am.Vote != nil {
			err = sn.Propose(am.Vote.View, am, "")
		} else {
			// Launch the announcement process
			err = sn.Announce(sn.ViewNo, am)
		}

		if err != nil {
			log.Errorln(err)
			cancelederr = err
			cancel()
		}
	}()

	// 1st Phase succeeded or connection error
	select {
	case _ = <-sn.commitsDone:
		// log time it took for first round to complete
		//firstRoundTime = time.Since(first)
		//sn.logFirstPhase(firstRoundTime)
		break
	case <-sn.closed:
		return errors.New("closed")
	case <-ctx.Done():
		log.Errorln(ctx.Err())
		if ctx.Err() == context.Canceled {
			return cancelederr
		}
		return errors.New("Really bad. Round did not finish commit phase and did not report network errors.")
	}

	// 2nd Phase succeeded or connection error
	select {
	case _ = <-sn.done:
		// log time it took for second round to complete
		//totalTime = time.Since(total)
		//sn.logSecondPhase(totalTime - firstRoundTime)
		//sn.logTotalTime(totalTime)
		return nil
	case <-sn.closed:
		return errors.New("closed")
	case <-ctx.Done():
		log.Errorln(ctx.Err())
		if ctx.Err() == context.Canceled {
			return cancelederr
		}
		return errors.New("Really bad. Round did not finish response phase and did not report network errors.")
	}
}

func (sn *Node) StartVotingRound(v *Vote) error {
	log.Println(sn.Name(), "start voting round")
	sn.nRounds = sn.LastSeenRound

	// during view changes, only accept view change related votes
	if sn.ChangingView && v.Vcv == nil {
		log.Println(sn.Name(), "start signing round: changingViewError")
		return ChangingViewError
	}

	sn.nRounds++
	v.Round = sn.nRounds
	v.Index = int(atomic.LoadInt64(&sn.LastSeenVote)) + 1
	v.Count = &Count{}
	v.Confirmed = false
	// only default fill-in view numbers when not prefilled
	if v.View == 0 {
		v.View = sn.ViewNo
	}
	if v.Av != nil && v.Av.View == 0 {
		v.Av.View = sn.ViewNo + 1
	}
	if v.Rv != nil && v.Rv.View == 0 {
		v.Rv.View = sn.ViewNo + 1
	}
	if v.Vcv != nil && v.Vcv.View == 0 {
		v.Vcv.View = sn.ViewNo + 1
	}
	return sn.StartAnnouncement(
		&AnnouncementMessage{Message: []byte("vote round"), Round: sn.nRounds, Vote: v})
}

func (sn *Node) StartSigningRound() error {
	sn.nRounds = sn.LastSeenRound

	// report view is being change, and sleep before retrying
	sn.viewmu.Lock()
	if sn.ChangingView {
		log.Println(sn.Name(), "start signing round: changingViewError")
		sn.viewmu.Unlock()
		return ChangingViewError
	}
	sn.viewmu.Unlock()

	sn.nRounds++
	// Adding timestamp
	ts := time.Now().UTC()
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, ts.Unix())
	return sn.StartAnnouncement(
		&AnnouncementMessage{Message: b.Bytes(), Round: sn.nRounds})
}

func NewNode(hn coconet.Host, suite abstract.Suite, random cipher.Stream) *Node {
	sn := &Node{Host: hn, suite: suite}
	msgSuite = suite
	sn.PrivKey = suite.Secret().Pick(random)
	sn.PubKey = suite.Point().Mul(nil, sn.PrivKey)

	sn.peerKeys = make(map[string]abstract.Point)
	sn.Rounds = make(map[int]*Round)

	sn.closed = make(chan error, 20)
	sn.done = make(chan int, 10)
	sn.commitsDone = make(chan int, 10)
	sn.viewChangeCh = make(chan string, 0)

	sn.FailureRate = 0
	h := fnv.New32a()
	h.Write([]byte(hn.Name()))
	seed := h.Sum32()
	sn.Rand = rand.New(rand.NewSource(int64(seed)))
	sn.Host.SetSuite(suite)
	sn.VoteLog = NewVoteLog()
	sn.Actions = make(map[int][]*Vote)
	sn.RoundsPerView = 0
	return sn
}

// Create new signing node that incorporates a given private key
func NewKeyedNode(hn coconet.Host, suite abstract.Suite, PrivKey abstract.Secret) *Node {
	sn := &Node{Host: hn, suite: suite, PrivKey: PrivKey}
	sn.PubKey = suite.Point().Mul(nil, sn.PrivKey)

	msgSuite = suite
	sn.peerKeys = make(map[string]abstract.Point)
	sn.Rounds = make(map[int]*Round)

	sn.closed = make(chan error, 20)
	sn.done = make(chan int, 10)
	sn.commitsDone = make(chan int, 10)
	sn.viewChangeCh = make(chan string, 0)

	sn.FailureRate = 0
	h := fnv.New32a()
	h.Write([]byte(hn.Name()))
	seed := h.Sum32()
	sn.Rand = rand.New(rand.NewSource(int64(seed)))
	sn.Host.SetSuite(suite)
	sn.VoteLog = NewVoteLog()
	sn.Actions = make(map[int][]*Vote)
	sn.RoundsPerView = 0
	return sn
}

func (sn *Node) ShouldIFail(phase string) bool {
	if sn.FailureRate > 0 {
		// If we were manually set to always fail
		if sn.Host.(*coconet.FaultyHost).IsDead() ||
			sn.Host.(*coconet.FaultyHost).IsDeadFor(phase) {
			// log.Println(sn.Name(), "dead for "+phase)
			return true
		}

		// If we were only given a probability of failing
		if p := sn.Rand.Int() % 100; p < sn.FailureRate {
			// log.Println(sn.Name(), "died for "+phase, "p", p, "with prob ", sn.FailureRate)
			return true
		}

	}

	return false
}

func (sn *Node) AddPeer(conn string, PubKey abstract.Point) {
	sn.Host.AddPeers(conn)
	sn.peerKeys[conn] = PubKey
}

func (sn *Node) Suite() abstract.Suite {
	return sn.suite
}

func (sn *Node) Done() chan int {
	return sn.done
}

func (sn *Node) LastRound() int {
	sn.roundmu.Lock()
	lsr := sn.LastSeenRound
	sn.roundmu.Unlock()
	return lsr
}

func (sn *Node) SetLastSeenRound(round int) {
	sn.LastSeenRound = round
}

func (sn *Node) CommitedFor(round *Round) bool {
	sn.roundLock.RLock()
	defer sn.roundLock.RUnlock()

	if round.Log.v != nil {
		return true
	}
	return false
}

// Cast on vote for Vote
func (sn *Node) AddVotes(Round int, v *Vote) {
	if v == nil {
		return
	}

	round := sn.Rounds[Round]
	cv := round.Vote.Count
	vresp := &VoteResponse{Name: sn.Name()}

	// accept what admin requested with x% probability
	// TODO: replace with non-probabilistic approach, maybe callback
	forProbability := 100
	sn.randmu.Lock()
	if p := sn.Rand.Int() % 100; p < forProbability {
		cv.For += 1
		vresp.Accepted = true
	} else {
		cv.Against += 1
	}
	sn.randmu.Unlock()

	// log.Infoln(sn.Name(), "added votes. for:", cv.For, "against:", cv.Against)

	// Generate signature on Vote with OwnVote *counted* in
	b, err := v.MarshalBinary()
	if err != nil {
		log.Fatal("Marshal Binary on Counted Votes failed")
	}
	rand := sn.suite.Cipher([]byte(sn.Name() + strconv.Itoa(Round)))
	vresp.Sig = ElGamalSign(sn.suite, rand, b, sn.PrivKey)

	// Add VoteResponse to Votes
	v.Count.Responses = append(v.Count.Responses, vresp)
	round.Vote = v
}

func intToByteSlice(Round int) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, Round)
	return buf.Bytes()
}

// *only* called by root node
func (sn *Node) SetAccountableRound(Round int) {
	// Create my back link to previous round
	sn.SetBackLink(Round)

	h := sn.suite.Hash()
	h.Write(intToByteSlice(Round))
	h.Write(sn.Rounds[Round].BackLink)
	sn.Rounds[Round].AccRound = h.Sum(nil)

	// here I could concatenate sn.Round after the hash for easy keeping track of round
	// todo: check this
}

func (sn *Node) UpdateTimeout(t ...time.Duration) {
	if len(t) > 0 {
		sn.SetTimeout(t[0])
	} else {
		tt := time.Duration(sn.Height)*sn.DefaultTimeout() + sn.DefaultTimeout()
		sn.SetTimeout(tt)
	}
}

func (sn *Node) SetBackLink(Round int) {
	prevRound := Round - 1
	sn.Rounds[Round].BackLink = hashid.HashId(make([]byte, hashid.Size))
	if prevRound >= FIRST_ROUND {
		// My Backlink = Hash(prevRound, sn.Rounds[prevRound].BackLink, sn.Rounds[prevRound].MTRoot)
		h := sn.suite.Hash()
		if sn.Rounds[prevRound] == nil {
			log.Errorln(sn.Name(), "not setting back link")
			return
		}
		h.Write(intToByteSlice(prevRound))
		h.Write(sn.Rounds[prevRound].BackLink)
		h.Write(sn.Rounds[prevRound].MTRoot)
		sn.Rounds[Round].BackLink = h.Sum(nil)
	}
}

func (sn *Node) GenSetPool() {
	var p sync.Pool
	p.New = NewSigningMessage
	sn.SetPool(&p)
}

func (sn *Node) SetTimeout(t time.Duration) {
	sn.timeLock.Lock()
	sn.timeout = t
	sn.timeLock.Unlock()
}

func (sn *Node) Timeout() time.Duration {
	sn.timeLock.RLock()
	t := sn.timeout
	sn.timeLock.RUnlock()
	return t
}

func (sn *Node) DefaultTimeout() time.Duration {
	return 5000 * time.Millisecond
}
