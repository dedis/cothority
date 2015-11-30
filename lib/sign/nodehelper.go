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
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/dbg"

	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/logutils"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
	"sync/atomic"
)

/*
This implements the helper Node-methods
 */

type Type int // used by other modules as coll_sign.Type

var ChangingViewError error = errors.New("In the process of changing view")

const (
// Default Signature involves creating Merkle Trees
	MerkleTree = iota
// Basic Signature removes all Merkle Trees
// Collective public keys are still created and can be used
	PubKey
// Basic Signature on aggregated votes
	Voter
)

type Node struct {
	coconet.Host

												  // Signing Node will Fail at FailureRate probability
	FailureRate         int
	FailAsRootEvery     int
	FailAsFollowerEvery int

	randmu              sync.Mutex
	Rand                *rand.Rand

	Type                Type
	Height              int

	HostList            []string

	suite               abstract.Suite
	PubKey              abstract.Point            // long lasting public key
	PrivKey             abstract.Secret           // long lasting private key

	nRounds             int
	Rounds              map[int]Round
	roundmu             sync.Mutex
	LastSeenRound       int                       // largest round number I have seen
	RoundsAsRoot        int                       // latest continuous streak of rounds with sn root

												  // Little hack for the moment where we keep the number of responses +
												  // commits for each round so we know when to pass down the messages to the
												  // round interfaces.(it was the role of the RoundMerkle before)
	RoundCommits        map[int][]*SigningMessage
	RoundResponses      map[int][]*SigningMessage

	AnnounceLock        sync.Mutex

												  // NOTE: reuse of channels via round-number % Max-Rounds-In-Mermory can be used
	roundLock           sync.RWMutex
	Message             []byte                    // for testing purposes
	peerKeys            map[string]abstract.Point // map of all peer public keys

	closed              chan error                // error sent when connection closed
	Isclosed            bool
	done                chan int                  // round number sent when round done
	commitsDone         chan int                  // round number sent when announce/commit phase done

	RoundsPerView       int
												  // "root" or "regular" are sent on this channel to
												  // notify the maker of the sn what role sn plays in the new view
	viewChangeCh        chan string
	ChangingView        bool                      // TRUE if node is currently engaged in changing the view
	viewmu              sync.Mutex
	ViewNo              int

	timeout             time.Duration
	timeLock            sync.RWMutex

	hbLock              sync.Mutex
	heartbeat           *time.Timer

												  // ActionsLock sync.Mutex
												  // Actions     []*VoteRequest

	VoteLog             *VoteLog                  // log of all confirmed votes, useful for replay
	LastSeenVote        int64                     // max of all Highest Votes we've seen, and our last commited vote
	LastAppliedVote     int64                     // last vote we have committed to our log

	Actions             map[int][]*Vote

												  // These are stored during the challenge phase so that they can
												  // be sent to the client during the SignatureBroadcast
	Proof               proof.Proof
	MTRoot              hashid.HashId             // the very root of the big Merkle Tree
	Messages            int                       // Number of messages to be signed received
	MessagesInRun       int                       // Total number of messages since start of run

	PeerStatus          StatusReturnMessage       // Actual status of children peers
	PeerStatusRcvd      int                       // How many peers sent status

	MaxWait             time.Duration             // How long the announcement phase can take
}

// Start listening for messages coming from parent(up)
func (sn *Node) Listen() error {
	if sn.Pool() == nil {
		sn.GenSetPool()
	}
	err := sn.ProcessMessages()
	return err
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
	dbg.Lvl2(sn.Name(), "Root for view", view)
	var hl []string
	if view == 0 {
		hl = sn.HostListOn(view)
	} else {
		// we might not have the host list for current view
		// safer to use the previous view's hostlist, always
		hl = sn.HostListOn(view - 1)
	}
	return hl[view % len(hl)]
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

func (sn *Node) StartAnnouncementWithWait(round Round, wait time.Duration) error {
	sn.AnnounceLock.Lock()
	sn.nRounds = sn.LastSeenRound

	// report view is being change, and sleep before retrying
	sn.viewmu.Lock()
	if sn.ChangingView {
		dbg.Lvl1(sn.Name(), "start signing round: changingViewError")
		sn.viewmu.Unlock()
		return ChangingViewError
	}
	sn.viewmu.Unlock()

	sn.nRounds++
	sn.Rounds[sn.nRounds] = round

	defer sn.AnnounceLock.Unlock()

	dbg.Lvl2("root", sn.Name(), "starting announcement round for round: ", sn.nRounds, "on view", sn.ViewNo)

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	var cancelederr error
	go func() {
		var err error
		// Launch the announcement process
		err = sn.Announce(&SigningMessage{
			Type:     Announcement,
			RoundNbr: sn.nRounds,
			ViewNbr:     sn.ViewNo,
			Am: &AnnouncementMessage{
				RoundType: round.GetType(),
				Message: make([]byte, 0),
			},
		})

		if err != nil {
			dbg.Lvl1(err)
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
		dbg.Lvl1(ctx.Err())
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
		dbg.Lvl2("Timeout:", ctx.Err())
		if ctx.Err() == context.Canceled {
			return cancelederr
		}
		return errors.New("Really bad. Round did not finish response phase and did not report network errors.")
	}
}

func (sn *Node) StartAnnouncement(round Round) error {
	return sn.StartAnnouncementWithWait(round, sn.MaxWait)
}

func NewNode(hn coconet.Host, suite abstract.Suite, random cipher.Stream) *Node {
	sn := &Node{Host: hn, suite: suite}
	msgSuite = suite
	sn.PrivKey = suite.Secret().Pick(random)
	sn.PubKey = suite.Point().Mul(nil, sn.PrivKey)

	sn.peerKeys = make(map[string]abstract.Point)

	sn.closed = make(chan error, 20)
	sn.done = make(chan int, 10)
	sn.commitsDone = make(chan int, 10)
	sn.viewChangeCh = make(chan string, 0)

	sn.RoundCommits = make(map[int][]*SigningMessage)
	sn.RoundResponses = make(map[int][]*SigningMessage)
	sn.FailureRate = 0
	h := fnv.New32a()
	h.Write([]byte(hn.Name()))
	seed := h.Sum32()
	sn.Rand = rand.New(rand.NewSource(int64(seed)))
	sn.Host.SetSuite(suite)
	sn.VoteLog = NewVoteLog()
	sn.Actions = make(map[int][]*Vote)
	sn.RoundsPerView = 0
	sn.Rounds = make(map[int]Round)
	sn.MaxWait = 50 * time.Second
	return sn
}

// Create new signing node that incorporates a given private key
func NewKeyedNode(hn coconet.Host, suite abstract.Suite, PrivKey abstract.Secret) *Node {
	sn := &Node{Host: hn, suite: suite, PrivKey: PrivKey}
	sn.PubKey = suite.Point().Mul(nil, sn.PrivKey)

	msgSuite = suite
	sn.peerKeys = make(map[string]abstract.Point)

	sn.closed = make(chan error, 20)
	sn.done = make(chan int, 10)
	sn.commitsDone = make(chan int, 10)
	sn.viewChangeCh = make(chan string, 0)

	sn.RoundCommits = make(map[int][]*SigningMessage)
	sn.RoundResponses = make(map[int][]*SigningMessage)

	sn.FailureRate = 0
	h := fnv.New32a()
	h.Write([]byte(hn.Name()))
	seed := h.Sum32()
	sn.Rand = rand.New(rand.NewSource(int64(seed)))
	sn.Host.SetSuite(suite)
	sn.VoteLog = NewVoteLog()
	sn.Actions = make(map[int][]*Vote)
	sn.RoundsPerView = 0
	sn.Rounds = make(map[int]Round)
	sn.MaxWait = 50 * time.Second
	return sn
}

func (sn *Node) ShouldIFail(phase string) bool {
	if sn.FailureRate > 0 {
		// If we were manually set to always fail
		if sn.Host.(*coconet.FaultyHost).IsDead() ||
		sn.Host.(*coconet.FaultyHost).IsDeadFor(phase) {
			dbg.Lvl2(sn.Name(), "dead for " + phase)
			return true
		}

		// If we were only given a probability of failing
		if p := sn.Rand.Int() % 100; p < sn.FailureRate {
			dbg.Lvl2(sn.Name(), "died for " + phase, "p", p, "with prob ", sn.FailureRate)
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

func (sn *Node) SetLastSeenRound(roundNbr int) {
	sn.LastSeenRound = roundNbr
}

func intToByteSlice(roundNbr int) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, roundNbr)
	return buf.Bytes()
}

func (sn *Node) UpdateTimeout(t ...time.Duration) {
	if len(t) > 0 {
		sn.SetTimeout(t[0])
	} else {
		tt := time.Duration(sn.Height) * sn.DefaultTimeout() + sn.DefaultTimeout()
		sn.SetTimeout(tt)
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

func (sn *Node) CloseAll(view int) error {
	dbg.Lvl2(sn.Name(), "received CloseAll on", view)

	// At the leaves
	if len(sn.Children(view)) == 0 {
		dbg.Lvl3(sn.Name(), "in CloseAll is root leaf")
	} else {
		dbg.Lvl3(sn.Name(), "in CloseAll is calling", len(sn.Children(view)), "children")

		// Inform all children of announcement
		messgs := make([]coconet.BinaryMarshaler, sn.NChildren(view))
		for i := range messgs {
			sm := SigningMessage{
				Type:         CloseAll,
				ViewNbr:         view,
				LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			}
			messgs[i] = &sm
		}
		ctx := context.TODO()
		if err := sn.PutDown(ctx, view, messgs); err != nil {
			return err
		}
	}
	dbg.Lvl3("Closing down shop", sn.Isclosed)
	sn.Close()
	return nil
}

func (sn *Node) PutUpError(view int, err error) {
	// dbg.Lvl4(sn.Name(), "put up response with err", err)
	// ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
	ctx := context.TODO()
	sn.PutUp(ctx, view, &SigningMessage{
		Type:         Error,
		ViewNbr:         view,
		LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
		Err:          &ErrorMessage{Err: err.Error()}})
}

// Getting actual View
func (sn *Node) GetView() int {
	return sn.ViewNo
}
