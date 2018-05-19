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
		c.subCommitment = make(chan StructCommitment, 2) //can send 2 commitments
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

//TODO R: think about 2-level trees (especially !p.IsRoot())
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
	var secret kyber.Scalar
	if !p.IsRoot() {
		go func() {
			log.Lvl3(p.ServerIdentity(), "starting verification in the background")
			verificationOk := p.verificationFn(p.Msg, p.Data)

			var personalStructCommitment StructCommitment
			secret, personalStructCommitment, err = p.getCommitment(verificationOk)
			if err != nil {
				log.Errorf("error while generating own commitment:", err)
			}
			p.ChannelCommitment <- personalStructCommitment
			log.Lvl3(p.ServerIdentity(), "verification done:", verificationOk)
		}()
	}

	if !p.IsLeaf() {
		// Only send commits if the node has children
		go func() {
			if errs := p.SendToChildrenInParallel(&announcement.Announcement); len(errs) > 0 {
				log.Lvl3(p.ServerIdentity(), "failed to send announcement to all children")
			}
		}()
	}

	// ----- Commitment & Challenge -----

	var challenge StructChallenge
	var committedChildren = make([]*onet.TreeNode, 0)
	var NRefusal = 0 // for the subleader
	var commitments = make([]StructCommitment, 0)
	var firstCommitmentSent = false
	var timedOut = false
	var t = time.After(p.Timeout / 2)

loop:
	for {
		select {
		case commitment, channelOpen := <-p.ChannelCommitment:
			if !channelOpen {
				return nil
			}
			if timedOut { //ignore new commits once time-out has been reached //TODO L: check if correct
				break
			}

			isOwnCommitment := commitment.TreeNode.ID.Equal(p.TreeNode().ID)

			if commitment.TreeNode.Parent != p.TreeNode() && !isOwnCommitment {
				log.Lvl2("received a Commitment from a node that is neither a children nor itself, ignored")
				break //discards it
			}

			if p.IsRoot() {
				// send commitment to super-protocol
				p.subCommitment <- commitment

				//deactivate timeout
				t = make(chan time.Time) //TODO T: see if should only do that on final answer

				committedChildren = []*onet.TreeNode{commitment.TreeNode}
			} else {
				if commitment.CoSiCommitment.Equal(p.suite.Point().Null()) { //refusal
					NRefusal++
				} else { //accepted
					if !isOwnCommitment {
						committedChildren = append(committedChildren, commitment.TreeNode)
					}
					commitments = append(commitments, commitment)
				}

				//TODO R: implement 0 threshold
				thresholdRefusal := (1 + len(p.Children()) - p.Threshold) + 1
				quickAnswer := !firstCommitmentSent &&
					(len(commitments) >= p.Threshold || // quick valid answer
						NRefusal >= thresholdRefusal) // quick refusal answer
				finalAnswer := len(commitments)+NRefusal == len(p.Children())+1

				if quickAnswer || finalAnswer || p.IsLeaf() {

					err = p.sendAggregatedCommitments(commitments, NRefusal)
					if err != nil {
						return err
					}

					//deactivate timeout if final commitment
					if firstCommitmentSent || p.IsLeaf() {
						t = make(chan time.Time)
					}

					firstCommitmentSent = true
				}

				//security check
				if len(commitments)+NRefusal > maxThreshold {
					log.Error(p.ServerIdentity(), "more commitments (", len(commitments),
						") and refusals (", NRefusal, ") than possible in subleader (", maxThreshold, ")")
				}
			}
		case challenge, channelOpen = <-p.ChannelChallenge:
			if !channelOpen {
				return nil
			}
			log.Lvl3(p.ServerIdentity(), "received challenge")

			//send challenge to children
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
			log.Error(p.ServerIdentity(), "timed out while waiting for commits, got", len(commitments), "commitments and", NRefusal, "refusals")

			//sending commits received
			err = p.sendAggregatedCommitments(commitments, NRefusal)
			if err != nil {
				return err
			}
			timedOut = true
		}
	}

	// ----- Response -----
	responses := make([]StructResponse, 0)

	// Second half of our time budget for the responses.
	timeout := time.After(p.Timeout / 2) //TODO T: do we really need a timeout for the responses?
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
		if secret != nil {
			// add own response
			personalResponse, err := cosi.Response(p.suite, p.Private(), secret, challenge.CoSiChallenge)
			if err != nil {
				return fmt.Errorf("error while generating own response: %s", err)
			}
			responses = append(responses, StructResponse{p.TreeNode(), Response{personalResponse}})
		}

		aggResponse, err := aggregateResponses(p.suite, responses)
		if err != nil {
			return err
		}
		err = p.SendToParent(&Response{aggResponse})
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *SubFtCosi) sendAggregatedCommitments(commitments []StructCommitment, NRefusal int) error {

	// aggregate commitments
	commitment, mask, err := aggregateCommitments(p.suite, p.Publics, commitments)
	if err != nil {
		return err
	}

	// send to parent
	err = p.SendToParent(&Commitment{commitment, mask.Mask(), NRefusal})
	if err != nil {
		return err
	}

	log.Lvl2(p.ServerIdentity(), "commitment sent with", mask.CountEnabled(), "accepted and", NRefusal, "refusals")

	return nil
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

func (p *SubFtCosi) getCommitment(accepts bool) (kyber.Scalar, StructCommitment, error) {

	emptyMask, err := cosi.NewMask(p.suite, p.Publics, nil)
	if err != nil {
		return nil, StructCommitment{}, err
	}

	structCommitment := StructCommitment{p.TreeNode(),
		Commitment{p.suite.Point().Null(), emptyMask.Mask(), 0}}

	var secret kyber.Scalar = nil
	if accepts {
		secret, structCommitment.CoSiCommitment = cosi.Commit(p.suite)
		var personalMask *cosi.Mask
		personalMask, err = cosi.NewMask(p.suite, p.Publics, p.Public())
		if err != nil {
			return secret, StructCommitment{}, err
		}
		structCommitment.Mask = personalMask.Mask()
	} else { //refuses
		structCommitment.NRefusal++
	}

	return secret, structCommitment, nil
}
