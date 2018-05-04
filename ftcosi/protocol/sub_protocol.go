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

	// ----- Commitment -----
	commitments := make([]StructCommitment, 0)
	if !p.IsLeaf() {
		// Only wait for commits if it's the root or the subleader.
		t := time.After(p.Timeout / 2)
	loop:
		for range p.Children() {
			select {
			case commitment, channelOpen := <-p.ChannelCommitment:
				if !channelOpen {
					return nil
				}
				commitments = append(commitments, commitment)
			case <-t:
				if p.IsRoot() {
					log.Error(p.ServerIdentity(), "timed out while waiting for subleader")
					p.subleaderNotResponding <- true
					return nil
				}
				log.Error(p.ServerIdentity(), "timed out while waiting for commits")
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
	var ok bool

	if p.IsRoot() {
		// send commitment to super-protocol
		if len(commitments) != 1 {
			return fmt.Errorf("root node in subprotocol should have received 1 commitment,"+
				"but received %d", len(commitments))
		}
		p.subCommitment <- commitments[0]
	} else {
		ok = <-verifyChan
		if !ok {
			log.Lvl2(p.ServerIdentity().Address, "verification failed, unsetting the mask")
		}

		// otherwise, compute personal commitment and send to parent
		var commitment kyber.Point
		var mask *cosi.Mask
		secret, commitment, mask, err = generateCommitmentAndAggregate(p.suite, p.TreeNodeInstance, p.Publics, commitments, ok)
		if err != nil {
			return err
		}

		// unset the mask if the verification failed and remove commitment
		var found bool
		if !ok {
			for i := range p.Publics {
				if p.Public().Equal(p.Publics[i]) {
					mask.SetBit(i, false)
					found = true
					break
				}
			}
		}
		if !ok && !found {
			return fmt.Errorf("%s was unable to find its own public key", p.ServerIdentity().Address)
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

	log.Lvl3(p.ServerIdentity(), "received challenge")
	go func() {
		if errs := p.multicastParallel(&challenge.Challenge, committedChildren...); len(errs) > 0 {
			log.Lvl3(p.ServerIdentity(), errs)
		}
	}()

	// ----- Response -----
	if p.IsLeaf() {
		p.ChannelResponse <- StructResponse{}
	}
	responses := make([]StructResponse, 0)

	// Second half of our time budget for the responses.
	timeout := time.After(p.Timeout / 2)
	for range committedChildren {
		select {
		case response, channelOpen := <-p.ChannelResponse:
			if !channelOpen {
				return nil
			}
			responses = append(responses, response)
		case <-timeout:
			log.Error(p.ServerIdentity(), "timeout while waiting for responses")
			break
		}
	}
	log.Lvl3(p.ServerIdentity(), "received all", len(responses), "response(s)")

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
			p.suite, p.TreeNodeInstance, responses, secret, challenge.Challenge.CoSiChallenge, ok)
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
