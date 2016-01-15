package sign

import (
	"bytes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/dedis/cothority/lib/dbg"

	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/lib/tree"
	"github.com/dedis/crypto/abstract"
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
	Host network.Host

	// Signing Node will Fail at FailureRate probability
	FailureRate         int
	FailAsRootEvery     int
	FailAsFollowerEvery int

	randmu sync.Mutex
	Rand   *rand.Rand

	Type   Type
	Height int

	suite   abstract.Suite
	PubKey  abstract.Point  // long lasting public key
	PrivKey abstract.Secret // long lasting private key

	nRounds       int
	Rounds        map[int]Round
	roundmu       sync.Mutex
	LastSeenRound int // largest round number I have seen
	RoundsAsRoot  int // latest continuous streak of rounds with sn root

	// Little hack for the moment where we keep the number of responses +
	// commits for each round so we know when to pass down the messages to the
	// round interfaces.(it was the role of the RoundMerkle before)
	RoundCommits   map[int][]*CommitmentMessage
	RoundResponses map[int][]*ResponseMessage

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

	VoteLog *VoteLog // log of all confirmed votes, useful for replay
	//LastSeenVote    int64    // max of all Highest Votes we've seen, and our last commited vote
	LastAppliedVote int64 // last vote we have committed to our log

	Actions map[int][]*Vote

	// These are stored during the challenge phase so that they can
	// be sent to the client during the SignatureBroadcast
	Proof         proof.Proof
	MTRoot        hashid.HashId // the very root of the big Merkle Tree
	Messages      int           // Number of messages to be signed received
	MessagesInRun int           // Total number of messages since start of run

	PeerStatus     StatusReturnMessage // Actual status of children peers
	PeerStatusRcvd int                 // How many peers sent status

	MaxWait time.Duration // How long the announcement phase can take
	// MsgChans used to Fanin all the messages received by many connections
	// peers
	MsgChans chan network.NetworkMessage
	// lists of name -> connections used by this node
	Conns     map[string]network.Conn
	connsLock sync.Mutex
	// Views are moved here (decoupled from network layer part)
	*tree.Views
	name string
}

func (sn *Node) Name() string {
	return sn.name
}

// Start listening for messages coming from parent(up)
// each time a connection request is made, we receive first its identity then
// we handle the message using HandleConn
func (sn *Node) Listen(addr string) error {
	fn := func(c network.Conn) {
		ctx := context.TODO()
		am, err := c.Receive(ctx)
		if err != nil || am.MsgType != Identity {
			dbg.Lvl2(sn.Name(), "Error receiving identity from connection", c.Remote())
		}
		id := am.Msg.(IdentityMessage)
		sn.connsLock.Lock()
		sn.Conns[id.PeerName] = c
		sn.connsLock.Unlock()
		dbg.Lvl3(sn.Name(), "Accepted Connection from", id.PeerName)
		sn.HandleConn(id.PeerName, c)
	}
	go sn.Host.Listen(addr, fn)
	// XXX Should it be here ?
	return sn.ProcessMessages()
}

// HandleConn receives message and pass it along the msgchannels
func (sn *Node) HandleConn(name string, c network.Conn) {
	for !sn.Isclosed {
		ctx := context.TODO()
		am, err := c.Receive(ctx)
		// XXX this is only a workaround. Need to find better ways to handle
		// error than an "IF" in nodeprotocol
		am.SetError(err)
		am.From = name
		sn.MsgChans <- am
	}
}

// Open will connect to the specified node and directly send him our identity
func (sn *Node) Open(name string) error {
	c, err := sn.Host.Open(name)
	if err != nil {
		return err
	}
	sn.connsLock.Lock()
	sn.Conns[name] = c
	sn.connsLock.Unlock()
	ctx := context.TODO()
	if err := c.Send(ctx, &IdentityMessage{sn.Name()}); err != nil {
		return err
	}
	go sn.HandleConn(name, c)
	return nil
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
		dbg.Lvl2("signing node: closing:", sn.Name())
		if err := sn.Host.Close(); err != nil {
			dbg.Lvl2(sn.Name(), "Close() error:", err)
		}
	}
	sn.Isclosed = true
	dbg.Lvl3(sn.Name(), "Closed connections")
	sn.hbLock.Unlock()
}

func (sn *Node) ViewChangeCh() chan string {
	return sn.viewChangeCh
}

// Returns name of node who should be the root for the next view
// round robin is used on the array of host names to determine the next root
func (sn *Node) RootFor(view int) string {
	dbg.Lvl2(sn.Name(), "Root for view", view)
	var hl *tree.PeerList
	if view == 0 {
		hl = sn.PeerList(view)
	} else {
		// we might not have the host list for current view
		// safer to use the previous view's hostlist, always
		hl = sn.PeerList(view - 1)
	}
	return hl.Peers[view%len(hl.Peers)].Name
}

func (sn *Node) SetFailureRate(v int) {
	sn.FailureRate = v
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

	dbg.Lvl2("root", sn.Name(), "starting announcement round for round:", sn.nRounds, "on view", sn.ViewNo)

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	var cancelederr error
	go func() {
		var err error
		// Launch the announcement process
		err = sn.Announce(&AnnouncementMessage{
			SigningMessage: &SigningMessage{
				RoundNbr: sn.nRounds,
				ViewNbr:  sn.ViewNo},
			RoundType: round.GetType(),
			Message:   make([]byte, 0),
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

func NewNode(suite abstract.Suite, name string, hn network.Host, views *tree.Views, random cipher.Stream) *Node {
	sn := &Node{Host: hn, suite: suite}
	sn.PrivKey = suite.Secret().Pick(random)
	sn.PubKey = suite.Point().Mul(nil, sn.PrivKey)

	sn.peerKeys = make(map[string]abstract.Point)

	sn.closed = make(chan error, 20)
	sn.done = make(chan int, 10)
	sn.commitsDone = make(chan int, 10)
	sn.viewChangeCh = make(chan string, 0)

	sn.RoundCommits = make(map[int][]*CommitmentMessage)
	sn.RoundResponses = make(map[int][]*ResponseMessage)
	sn.FailureRate = 0
	h := fnv.New32a()
	// TODO this is bad !
	h.Write([]byte(name))
	seed := h.Sum32()
	sn.Rand = rand.New(rand.NewSource(int64(seed)))
	sn.VoteLog = NewVoteLog()
	sn.Actions = make(map[int][]*Vote)
	sn.RoundsPerView = 0
	sn.Rounds = make(map[int]Round)
	sn.MaxWait = 50 * time.Second
	sn.MsgChans = make(chan network.NetworkMessage)
	sn.Conns = make(map[string]network.Conn)
	sn.connsLock = sync.Mutex{}
	sn.Views = views
	sn.name = name
	return sn
}

// Create new signing node that incorporates a given private key
func NewKeyedNode(suite abstract.Suite, name string, hn network.Host, views *tree.Views) *Node {
	sn := &Node{Host: hn, suite: suite}
	sn.PrivKey = views.View(0).Secret
	sn.PubKey = views.View(0).Public

	sn.peerKeys = make(map[string]abstract.Point)

	sn.closed = make(chan error, 20)
	sn.done = make(chan int, 10)
	sn.commitsDone = make(chan int, 10)
	sn.viewChangeCh = make(chan string, 0)

	sn.RoundCommits = make(map[int][]*CommitmentMessage)
	sn.RoundResponses = make(map[int][]*ResponseMessage)

	sn.FailureRate = 0
	h := fnv.New32a()
	h.Write([]byte(name))
	seed := h.Sum32()
	sn.Rand = rand.New(rand.NewSource(int64(seed)))
	sn.VoteLog = NewVoteLog()
	sn.Actions = make(map[int][]*Vote)
	sn.RoundsPerView = 0
	sn.Rounds = make(map[int]Round)
	sn.MaxWait = 50 * time.Second
	sn.MsgChans = make(chan network.NetworkMessage)
	sn.Conns = make(map[string]network.Conn)
	sn.connsLock = sync.Mutex{}
	sn.Views = views
	sn.name = name
	return sn
}

func (sn *Node) ShouldIFail(phase string) bool {
	// XXX not used for the moment
	// have to switch to network.Host
	/* if sn.FailureRate > 0 {*/
	//// If we were manually set to always fail
	//if sn.Host.(*coconet.FaultyHost).IsDead() ||
	//sn.Host.(*coconet.FaultyHost).IsDeadFor(phase) {
	//dbg.Lvl2(sn.Name(), "dead for "+phase)
	//return true
	//}

	//// If we were only given a probability of failing
	//if p := sn.Rand.Int() % 100; p < sn.FailureRate {
	//dbg.Lvl2(sn.Name(), "died for "+phase, "p", p, "with prob", sn.FailureRate)
	//return true
	//}

	//}

	return false
}

func (sn *Node) AddPeer(conn string, PubKey abstract.Point) {
	// it does not connect so what it is used for
	//sn.Host.AddPeers(conn)
	sn.peerKeys[conn] = PubKey
}

// Returns true if this node is the parent of the child
func (sn *Node) ParentOf(view int, child string) bool {
	if v := sn.View(view); v != nil {
		return v.ParentOf(child)
	}
	return false
}

// Are this node root this view ?
func (sn *Node) Root(view int) bool {
	if v := sn.View(view); v != nil {
		return v.Root()
	}
	return false
}

// are we leaf on this view ?
func (sn *Node) Leaf(view int) bool {
	if v := sn.View(view); v != nil {
		return v.Leaf()
	}
	return false
}

// Same as ParentOf but for children ....
func (sn *Node) ChildOf(view int, parent string) bool {
	if v := sn.View(view); v != nil {
		return v.ChildOf(parent)
	}
	return false
}

// PutDownAll puts the msg down the tree (Sending to children)
// TODO make it work with []network.ProtocolMessage since casting does not work
// for array/slices
func (sn *Node) PutDownAll(ctx context.Context, view int, msg ...network.ProtocolMessage) error {
	children := sn.Children(view)
	if children == nil {
		return fmt.Errorf("PutDownAll : No views %d", view)
	}
	if len(children) != len(msg) {
		return fmt.Errorf("PutDownAll  received different numbers of message", len(msg), " vs children", len(children))
	}
	// Every children of this node
	for i, ch := range children {
		// look if we have indeed a connection
		if c, ok := sn.Conns[ch.Name()]; ok {
			// then send it
			if err := c.Send(ctx, msg[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// PutDown only send a message down to a child
func (sn *Node) PutDown(ctx context.Context, view int, name string, msg network.ProtocolMessage) error {
	// Verify that it is indeed a child
	if v := sn.View(view); v == nil {
		fmt.Errorf("PutDown no view for %d", view)
	} else if !v.ParentOf(name) {
		fmt.Errorf("PutDown %s is not a children for view %d", name, view)
	}
	sn.connsLock.Lock()
	// if we have a connection ready for this child
	c, ok := sn.Conns[name]
	sn.connsLock.Unlock()
	if !ok {
		return fmt.Errorf("No connection to %s", name)
	}
	// Send the stuff !
	return c.Send(ctx, msg)
}

// PutUp send the message to the parent of this node
func (sn *Node) PutUp(ctx context.Context, view int, msg network.ProtocolMessage) error {
	var c network.Conn
	var ok bool
	var err error
	sn.connsLock.Lock()
	if node := sn.Parent(view); node == nil {
		err = fmt.Errorf("PutUp Got no parent for this view %d", view)
	} else if c, ok = sn.Conns[node.Name()]; !ok {
		err = fmt.Errorf("PutUp got no connection to parent %s in view %d", node.Name(), view)
	}
	sn.connsLock.Unlock()
	if err != nil {
		return err
	}
	return c.Send(ctx, msg)
}

func (sn *Node) PutTo(ctx context.Context, name string, msg network.ProtocolMessage) error {
	var c network.Conn
	var ok bool
	var err error
	sn.connsLock.Unlock()
	if c, ok = sn.Conns[name]; !ok {
		err = fmt.Errorf("PutTo given unknown peer name %s", name)
	}
	sn.connsLock.Unlock()
	if err != nil {
		return err
	}
	return c.Send(ctx, msg)
}

// ConnectParent will contact the parent in this view. If we are the root,
// it will return an error as we are not supposed to do that.
func (sn *Node) ConnectParent(view int) error {
	var parent *tree.Node
	if sn.Root(view) {
		return fmt.Errorf("%s SHould NOT connect to parent since he is root", sn.Name())
	}
	if parent = sn.Parent(view); parent == nil {
		return fmt.Errorf("Could not connect to parent in view %d", view)
	}
	return sn.Open(parent.Name())
}

func (sn *Node) Suite() abstract.Suite {
	return sn.suite
}

func (sn *Node) Done() chan int {
	return sn.done
}

func (sn *Node) WaitChildrenConnections(view int) {
	done := make(chan bool)
	go func() {
		for {
			var connected int
			sn.connsLock.Lock()
			/*         if sn.Root(view) {*/
			//fmt.Println(sn.Name(), "View.Children=", sn.Children(view))
			//fmt.Println(sn.Name(), "Conns =", sn.Conns)
			//}

			for _, c := range sn.Children(view) {
				if _, ok := sn.Conns[c.Name()]; ok {
					connected++
				}
			}
			sn.connsLock.Unlock()
			if connected == len(sn.Children(view)) {
				done <- true
				return
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	select {
	case <-done:
		close(done)
		return
	case <-time.After(30 * time.Second):
		close(done)
		dbg.Fatal(sn.Name(), "Children not connected after 30 secs")
	}
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
		tt := time.Duration(sn.Height)*sn.DefaultTimeout() + sn.DefaultTimeout()
		sn.SetTimeout(tt)
	}
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
		messgs := make([]*CloseAllMessage, sn.NChildren(view))
		for i := range messgs {
			sm := CloseAllMessage{
				ViewNbr: view,
			}
			messgs[i] = &sm
		}
		ctx := context.TODO()
		if err := sn.PutDownAll(ctx, view, messgs); err != nil {
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
	sn.PutUp(ctx, view, &ErrorMessage{
		SigningMessage: &SigningMessage{
			ViewNbr: view},
		Err: err.Error()})
}

// Getting actual View
func (sn *Node) GetView() int {
	return sn.ViewNo
}
