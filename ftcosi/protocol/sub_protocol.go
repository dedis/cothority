package protocol

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/kyber.v2/sign/cosi"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
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
	log.Lvl3(p.ServerIdentity().Address, "received announcement")
	p.Publics = announcement.Publics
	p.Timeout = announcement.Timeout
	p.Msg = announcement.Msg
	p.Data = announcement.Data
	var err error

	// start the verification in background if I'm not the root because
	// root does the verification in the main protocol
	verifyChan := make(chan bool, 1)
	if !p.IsRoot() {
		go func() {
			log.Lvl3(p.ServerIdentity().Address, "starting verification")
			verifyChan <- p.verificationFn(p.Msg, p.Data)
		}()
	}

	if errs := p.SendToChildrenInParallel(&announcement.Announcement); len(errs) > 0 {
		log.Lvl3(p.ServerIdentity().Address, "failed to send announcement to all children")
	}

	// ----- Commitment -----
	commitments := make([]StructCommitment, 0)
	if p.IsRoot() {
		select { // one commitment expected from super-protocol
		case commitment, channelOpen := <-p.ChannelCommitment:
			if !channelOpen {
				return nil
			}
			commitments = append(commitments, commitment)
		case <-time.After(p.Timeout):
			// the timeout here should be shorter than the main protocol timeout
			// because main protocol waits on the channel below
			p.subleaderNotResponding <- true
			return nil
		}
	} else {
		// the timeout should be shorter than the timeout for receiving
		// commits above (i.e. p.Timeout), hence it is reduced
		t := time.After(p.Timeout / 2)
	loop:
		// note that this section will not execute if it's on the leaf
		for range p.Children() {
			select {
			case commitment, channelOpen := <-p.ChannelCommitment:
				if !channelOpen {
					return nil
				}
				commitments = append(commitments, commitment)
			case <-t:
				break loop
			}
		}
	}

	committedChildren := make([]*onet.TreeNode, 0)
	for _, commitment := range commitments {
		if commitment.TreeNode.Parent != p.TreeNode() {
			return errors.New("received a Commitment from a non-Children node")
		}
		committedChildren = append(committedChildren, commitment.TreeNode)
	}
	log.Lvl3(p.ServerIdentity().Address, "finished receiving commitments, ", len(commitments), "commitment(s) received")

	var secret kyber.Scalar

	if p.IsRoot() {
		// send commitment to super-protocol
		if len(commitments) != 1 {
			return fmt.Errorf("root node in subprotocol should have received 1 commitment,"+
				"but received %d", len(commitments))
		}
		p.subCommitment <- commitments[0]
	} else {
		// do not commit if the verification does not succeed
		if !<-verifyChan {
			log.Lvl2(p.ServerIdentity().Address, "verification failed, terminating")
			return nil
		}

		// otherwise, compute personal commitment and send to parent
		var commitment kyber.Point
		var mask *cosi.Mask
		secret, commitment, mask, err = generateCommitmentAndAggregate(p.suite, p.TreeNodeInstance, p.Publics, commitments)
		if err != nil {
			return err
		}
		err = p.SendToParent(&Commitment{commitment, mask.Mask()})
		if err != nil {
			return err
		}
	}

	// ----- Challenge -----
	challenge, channelOpen := <-p.ChannelChallenge // from the leader
	if !channelOpen {
		return nil
	}

	log.Lvl3(p.ServerIdentity().Address, "received challenge")
	if errs := p.Multicast(&challenge.Challenge, committedChildren...); len(errs) > 0 {
		log.Lvl3(p.ServerIdentity().Address, "")
	}

	// ----- Response -----
	if p.IsLeaf() {
		p.ChannelResponse <- StructResponse{}
	}
	responses := make([]StructResponse, 0)

	for range committedChildren {
		response, channelOpen := <-p.ChannelResponse
		if !channelOpen {
			return nil
		}
		responses = append(responses, response)
	}
	log.Lvl3(p.ServerIdentity().Address, "received all", len(responses), "response(s)")

	if p.IsRoot() {
		// send response to super-protocol
		if len(responses) != 1 {
			return fmt.Errorf(
				"root node in subprotocol should have received 1 response, but received %v",
				len(commitments))
		}
		p.subResponse <- responses[0]
	} else {
		// generate own response and send to parent
		response, err := generateResponse(
			p.suite, p.TreeNodeInstance, responses, secret, challenge.Challenge.CoSiChallenge)
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
	log.Lvl3(p.ServerIdentity().Address, "Starting subCoSi")
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

	announcement := StructAnnouncement{
		p.TreeNode(),
		Announcement{p.Msg, p.Data, p.Publics, p.Timeout},
	}
	p.ChannelAnnouncement <- announcement
	return nil
}
