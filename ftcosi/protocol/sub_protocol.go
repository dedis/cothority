package protocol

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

func init() {
	GlobalRegisterDefaultProtocols()
}

// SubFtCosi holds the different channels used to receive the different protocol messages.
type SubFtCosi struct {
	*onet.TreeNodeInstance
	Publics        []kyber.Point
	Msg            []byte
	Data           []byte
	Timeout        time.Duration
	Threshold      int
	stoppedOnce    sync.Once
	verificationFn VerificationFn
	suite          cosi.Suite

	// protocol/subprotocol channels
	// these are used to communicate between the subprotocol and the main protocol
	subleaderNotResponding chan bool
	subCommitment          chan StructCommitment
	subResponse            chan StructResponse

	// internodes channels
	ChannelAnnouncement chan StructAnnouncement
	ChannelCommitment   chan StructCommitment
	ChannelChallenge    chan StructChallenge
	ChannelResponse     chan StructResponse
}

// NewDefaultSubProtocol is the default sub-protocol function used for registration
// with an always-true verification.
func NewDefaultSubProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewSubFtCosi(n, vf, cothority.Suite)
}

// NewSubFtCosi is used to define the subprotocol and to register
// the channels where the messages will be received.
func NewSubFtCosi(n *onet.TreeNodeInstance, vf VerificationFn, suite cosi.Suite) (onet.ProtocolInstance, error) {

	// tests if it's a three level tree
	moreThreeLevel := false
	n.Tree().Root.Visit(0, func(depth int, n *onet.TreeNode) {
		if depth > 2 {
			moreThreeLevel = true
		}
	})
	if moreThreeLevel {
		return nil, fmt.Errorf("subFtCosi launched with a more than three level tree")
	}

	c := &SubFtCosi{
		TreeNodeInstance: n,
		verificationFn:   vf,
		suite:            suite,
	}

	if n.IsRoot() {
		c.subleaderNotResponding = make(chan bool)
		c.subCommitment = make(chan StructCommitment)
		c.subResponse = make(chan StructResponse)
	}

	for _, channel := range []interface{}{
		&c.ChannelAnnouncement,
		&c.ChannelCommitment,
		&c.ChannelChallenge,
		&c.ChannelResponse,
	} {
		err := c.RegisterChannel(channel)
		if err != nil {
			return nil, errors.New("couldn't register channel: " + err.Error())
		}
	}
	err := c.RegisterHandler(c.HandleStop)
	if err != nil {
		return nil, errors.New("couldn't register stop handler: " + err.Error())
	}
	return c, nil
}

// Shutdown stops the protocol
func (p *SubFtCosi) Shutdown() error {
	p.stoppedOnce.Do(func() {
		close(p.ChannelAnnouncement)
		close(p.ChannelCommitment)
		close(p.ChannelChallenge)
		close(p.ChannelResponse)
	})
	return nil
}

// Dispatch is the main method of the subprotocol, running on each node and handling the messages in order
func (p *SubFtCosi) Dispatch() error {
	defer p.Done()

	// ----- Announcement -----
	announcement, channelOpen := <-p.ChannelAnnouncement
	if !channelOpen {
		return nil
	}
	log.Lvl3(p.ServerIdentity(), "received announcement")
	p.Publics = announcement.Publics
	p.Timeout = announcement.Timeout
	if !p.IsRoot() {
		// We'll be only waiting on the root and the subleaders. The subleaders
		// only have half of the time budget of the root.
		p.Timeout /= 2
	}
	p.Msg = announcement.Msg
	p.Data = announcement.Data
	p.Threshold = announcement.Threshold

	maxThreshold := p.Tree().Size() - 1
	if p.Threshold > maxThreshold {
		return fmt.Errorf("threshold %d bigger than the maximum of commitments this subtree can gather (%d)", p.Threshold, maxThreshold)
	}

	var err error

	// start the verification in background if I'm not the root because
	// root does the verification in the main protocol
	verifyChan := make(chan bool, 1)
	if !p.IsRoot() {
		go func() {
			log.Lvl3(p.ServerIdentity(), "starting verification")
			verifyChan <- p.verificationFn(p.Msg, p.Data)
		}()
	}

	if !p.IsLeaf() {
		// Only send commits if it's the root node or the subleader.
		go func() {
			if errs := p.SendToChildrenInParallel(&announcement.Announcement); len(errs) > 0 {
				log.Lvl3(p.ServerIdentity(), "failed to send announcement to all children")
			}
		}()
	}

	// ----- Commitment & Challenge -----

	var challenge StructChallenge
	committedChildren := make([]*onet.TreeNode, 0)
	commitments := make([]StructCommitment, 0)              // for the subleader
	ThresholdRefusal := 1 + len(p.Children()) - p.Threshold // for the subleader
	NRefusal := 0                                           // for the subleader
	var secret kyber.Scalar
	var verificationOk bool
	firstCommitmentSent := false
	t := time.After(p.Timeout / 2)

	if p.IsLeaf() {
		verificationOk = <-verifyChan
		if !verificationOk {
			log.Lvl2(p.ServerIdentity(), "verification failed, unsetting the mask")
		}

		secret, err = p.sendAgregatedCommitments(verificationOk, commitments, 0)
		if err != nil {
			return err
		}
		t = make(chan time.Time) //deactivate timeout
	}

loop:
	for {
		select {
		case commitment, channelOpen := <-p.ChannelCommitment:
			if !channelOpen {
				return nil
			}
			if commitment.TreeNode.Parent != p.TreeNode() {
				log.Lvl2("received a Commitment from a non-Children node")
				break //discards it
			}
			if p.IsRoot() {
				// send commitment to super-protocol
				p.subCommitment <- commitment

				//deactivate timeout
				t = make(chan time.Time) //TODO: see if should only do that on final answer

				committedChildren = []*onet.TreeNode{commitment.TreeNode}
			} else {
				if commitment.CoSiCommitment.Equal(p.suite.Point().Null()) { //refusal
					NRefusal++

					if NRefusal == ThresholdRefusal-1 {
						verificationOk = <-verifyChan
						if !verificationOk {
							//NRefusal++
						}
						verifyChan <- verificationOk // send back for other uses
					}
				} else { //accepted
					committedChildren = append(committedChildren, commitment.TreeNode)
					commitments = append(commitments, commitment)

					if len(commitments) == p.Threshold-1 {
						verificationOk = <-verifyChan
						if verificationOk {
							//NRefusal++
						}
						verifyChan <- verificationOk // send back for other uses
					}
				}

				//TODO:implement 0 threshold
				if (!firstCommitmentSent &&
					(len(commitments)+1 >= p.Threshold || // quick answer
						NRefusal > ThresholdRefusal)) || // quick refusal answer
					len(commitments)+NRefusal == len(p.Children()) { // final answer

					secret, err = p.sendAgregatedCommitments(verificationOk, commitments, NRefusal)
					if err != nil {
						return err
					}

					firstCommitmentSent = true
				}
				if len(commitments)+NRefusal > len(p.Children()) {
					log.Error(p.ServerIdentity(), "more commitments and refusal than number of children in subleader")
				}
			}
		case challenge, channelOpen = <-p.ChannelChallenge:
			if !channelOpen {
				return nil
			}
			log.Lvl3(p.ServerIdentity(), "received challenge")

			go func() {
				if errs := p.multicastParallel(&challenge.Challenge, committedChildren...); len(errs) > 0 {
					log.Lvl3(p.ServerIdentity(), errs)
				}
			}()

			break loop
		case <-t:
			if p.IsRoot() {
				log.Error(p.ServerIdentity(), "timed out while waiting for subleader commitment")
				p.subleaderNotResponding <- true
				return nil
			}
			log.Error(p.ServerIdentity(), "timed out while waiting for commits, got", len(commitments), "commitments")
			//sending commits received
			secret, err = p.sendAgregatedCommitments(verificationOk, commitments, NRefusal) //TODO: note that can still send final answer
			if err != nil {
				return err
			}
		}
	}
	//log.Lvl3(p.ServerIdentity(), "finished receiving commitments, ", len(commitments), "commitment(s) received")

	// ----- Response -----
	responses := make([]StructResponse, 0)

	// Second half of our time budget for the responses.
	timeout := time.After(p.Timeout / 2) //TODO: do we really need a timeout for the responses?
	for range committedChildren {
		select {
		case response, channelOpen := <-p.ChannelResponse:
			if !channelOpen {
				return nil
			}
			responses = append(responses, response)
		case <-timeout:
			log.Error(p.ServerIdentity(), "timeout while waiting for responses")
		}
	}
	log.Lvl3(p.ServerIdentity(), "received all", len(responses), "response(s)")

	if p.IsRoot() {
		// send response to super-protocol
		if len(responses) != 1 {
			return fmt.Errorf(
				"root node in subprotocol should have received 1 response, but received %v",
				len(responses))
		}
		p.subResponse <- responses[0]
	} else {
		// generate own response and send to parent
		response, err := generateResponse(
			p.suite, p.TreeNodeInstance, responses, secret, challenge.Challenge.CoSiChallenge, verificationOk)
		if err != nil {
			return err
		}
		err = p.SendToParent(&Response{response})
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *SubFtCosi) sendAgregatedCommitments(ok bool, commitments []StructCommitment,
	NRefusal int) (secret kyber.Scalar, err error) {

	// compute personal commitment
	var commitment kyber.Point
	var mask *cosi.Mask
	secret, commitment, mask, err = generateAggregatedCommitment(p.suite, p.TreeNodeInstance,
		p.Publics, commitments, ok)
	if err != nil {
		return nil, err
	}

	// send to parent
	err = p.SendToParent(&Commitment{commitment, mask.Mask(), NRefusal})
	if err != nil {
		return nil, err
	}

	return secret, nil
}

// HandleStop is called when a Stop message is send to this node.
// It broadcasts the message to all the nodes in tree and each node will stop
// the protocol by calling p.Done.
func (p *SubFtCosi) HandleStop(stop StructStop) error {
	defer p.Done()
	if p.IsRoot() {
		p.Broadcast(&stop.Stop)
	}
	return nil
}

// Start is done only by root and starts the subprotocol
func (p *SubFtCosi) Start() error {
	log.Lvl3(p.ServerIdentity(), "Starting subCoSi")
	if p.Msg == nil {
		return errors.New("subprotocol does not have a proposal msg")
	}
	if p.Data == nil {
		return errors.New("subprotocol does not have data, it can be empty but cannot be nil")
	}
	if p.Publics == nil || len(p.Publics) < 1 {
		return errors.New("subprotocol has invalid public keys")
	}
	if p.verificationFn == nil {
		return errors.New("subprotocol has an empty verification fn")
	}
	if p.Timeout < 10*time.Nanosecond {
		return errors.New("unrealistic timeout")
	}

	if p.Threshold > p.Tree().Size() {
		return errors.New("threshold bigger than number of nodes in subtree")
	}

	if p.Threshold == 0 {
		log.Lvl3("no threshold specified, using \"as much as possible\" policy")
	}

	announcement := StructAnnouncement{
		p.TreeNode(),
		Announcement{p.Msg, p.Data, p.Publics, p.Timeout, p.Threshold},
	}
	p.ChannelAnnouncement <- announcement
	return nil
}

// multicastParallel can be moved to onet.TreeNodeInstance once it shows
// promise.
func (p *SubFtCosi) multicastParallel(msg interface{}, nodes ...*onet.TreeNode) []error {
	var errs []error
	eMut := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, node := range nodes {
		name := node.Name()
		wg.Add(1)
		go func(n2 *onet.TreeNode) {
			if err := p.SendTo(n2, msg); err != nil {
				eMut.Lock()
				errs = append(errs, errors.New(name+": "+err.Error()))
				eMut.Unlock()
			}
			wg.Done()
		}(node)
	}
	wg.Wait()
	return errs
}
