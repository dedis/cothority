package protocol

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/sign/bls"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

func init() {
	GlobalRegisterDefaultProtocols()
}

// sub_protocol is run by each sub-leader and each node once, and n times by
// the root leader, where n is the number of sub-leader.

// SubBlsFtCosi holds the different channels used to receive the different protocol messages.
type SubBlsFtCosi struct {
	*onet.TreeNodeInstance
	Msg            []byte
	Data           []byte
	Timeout        time.Duration
	Threshold      int
	stoppedOnce    sync.Once
	verificationFn VerificationFn
	suite          *pairing.SuiteBn256
	startChan      chan bool

	// protocol/subprotocol channels
	// these are used to communicate between the subprotocol and the main protocol
	subleaderNotResponding chan bool
	subResponse            chan StructResponse

	// internodes channels
	ChannelAnnouncement chan StructAnnouncement
	ChannelResponse     chan StructResponse
	ChannelRefusal      chan StructRefusal
}

// NewDefaultSubProtocol is the default sub-protocol function used for registration
// with an always-true verification.
func NewDefaultSubProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewSubBlsFtCosi(n, vf, pairing.NewSuiteBn256())
}

// NewSubBlsFtCosi is used to define the subprotocol and to register
// the channels where the messages will be received.
func NewSubBlsFtCosi(n *onet.TreeNodeInstance, vf VerificationFn, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	// tests if it's a three level tree
	moreThreeLevel := false
	n.Tree().Root.Visit(0, func(depth int, n *onet.TreeNode) {
		if depth > 2 {
			moreThreeLevel = true
		}
	})
	if moreThreeLevel {
		return nil, fmt.Errorf("subBlsFtCosi launched with a more than three level tree")
	}

	c := &SubBlsFtCosi{
		TreeNodeInstance: n,
		verificationFn:   vf,
		suite:            suite,
		startChan:        make(chan bool, 1),
	}

	if n.IsRoot() {
		c.subleaderNotResponding = make(chan bool, 1)
		c.subResponse = make(chan StructResponse, 1)
	}

	err := c.RegisterChannels(&c.ChannelAnnouncement, &c.ChannelResponse, &c.ChannelRefusal)
	if err != nil {
		return nil, errors.New("couldn't register channels: " + err.Error())
	}
	err = c.RegisterHandler(c.HandleStop)
	if err != nil {
		return nil, errors.New("couldn't register stop handler: " + err.Error())
	}
	return c, nil
}

// Dispatch runs the protocol for each node in the protocol acting according
// to its type
func (p *SubBlsFtCosi) Dispatch() error {
	defer p.Done()

	// Send announcement to start sending signatures
	if p.IsRoot() {
		return p.dispatchRoot()
	} else if p.Parent().Equal(p.Root()) {
		return p.dispatchSubLeader()
	}

	return p.dispatchLeaf()
}

func (p *SubBlsFtCosi) waitAnnouncement(parent *onet.TreeNode) (*Announcement, bool) {
	var a *Announcement
	// Keep looping until the correct announcement to prevent
	// an attacker from killing the protocol with false message
	for a == nil {
		msg, ok := <-p.ChannelAnnouncement
		if !ok {
			return nil, ok
		}
		if parent.Equal(msg.TreeNode) {
			a = &msg.Announcement
		}
	}

	p.Msg = a.Msg
	p.Data = a.Data
	p.Timeout = a.Timeout / 2
	p.Threshold = a.Threshold

	return a, true
}

func (p *SubBlsFtCosi) checkFailureThreshold(numFailure int) bool {
	if numFailure == 0 {
		return false
	}

	return numFailure >= len(p.Roster().List)-p.Threshold
}

// dispatchRoot takes care of sending announcements to the children and
// waits for the response with the signatures of the children
func (p *SubBlsFtCosi) dispatchRoot() error {
	defer func() {
		err := p.Broadcast(&Stop{})
		if err != nil {
			log.Error("error while broadcasting stopping message:", err)
		}
	}()

	// make sure we're ready to go
	hasStarted := <-p.startChan
	if !hasStarted {
		return nil
	}

	// Only one child anyway
	err := p.SendToChildren(&Announcement{
		Msg:       p.Msg,
		Data:      p.Data,
		Timeout:   p.Timeout,
		Threshold: p.Threshold,
	})
	if err != nil {
		// Only log what happened so we can try to finish the protocol
		// (e.g. one child is offline)
		log.Warnf("Error when broadcasting to children: %s", err.Error())
	}

	select {
	case reply, ok := <-p.ChannelResponse:
		if !ok {
			return nil
		}

		// Transfer the response to the parent protocol
		p.subResponse <- reply
	case <-time.After(p.Timeout):
		// It might be only the subleader then we send a notification
		// to let the parent protocol take actions
		log.Warn(p.ServerIdentity(), "timed out while waiting for subleader response")
		p.subleaderNotResponding <- true
	}

	return nil
}

// dispatchSubLeader takes care of synchronizing the children
// responses and aggregate them to eventually send that to
// the root
func (p *SubBlsFtCosi) dispatchSubLeader() error {
	a, ok := p.waitAnnouncement(p.Root())
	if !ok {
		return nil // channel closed
	}

	errs := p.SendToChildrenInParallel(a)
	if len(errs) > 0 {
		log.Error(errs)
	}

	responses := make(ResponseMap)
	for _, c := range p.Children() {
		// Accept response for those identities only
		responses[c.ID] = nil
	}

	own, err := p.makeResponse()
	if ok := p.verificationFn(p.Msg, p.Data); ok {
		log.Lvlf3("Subleader %v signed", p.ServerIdentity())
		responses[p.TreeNode().ID] = own
	}

	timeout := time.After(p.Timeout)
	done := 0
	for done < len(p.Children()) {
		select {
		case reply, ok := <-p.ChannelResponse:
			if !ok {
				return nil
			}

			r, ok := responses[reply.ID]
			if !ok {
				log.Warnf("Got a message from an unknown node %v", reply.ServerIdentity.ID)
			} else if r == nil {
				public := p.RosterServicePublic(reply.ServerIdentity)
				if err := bls.Verify(p.suite, public, p.Msg, reply.Signature); err == nil {
					responses[reply.ID] = &reply.Response
					done++
				}
			} else {
				log.Warnf("Duplicate message from %v", reply.ServerIdentity.ID)
			}
		case reply, ok := <-p.ChannelRefusal:
			if !ok {
				return nil
			}

			serviceName := onet.ServiceFactory.Name(p.Token().ServiceID)

			if err := bls.Verify(p.suite, reply.ServerIdentity.ServicePublic(serviceName), reply.Nonce, reply.Signature); err == nil {
				// The child gives an empty signature as a mark of refusal
				responses[reply.ID] = &Response{}
				done++
			} else {
				log.Warnf("Tentative to send a unsigned refusal from %v", reply.ServerIdentity.ID)
			}
		case <-timeout:
			// Use whatever we received until then to try to finish
			// the protocol
			done = len(p.Children())
		}
	}

	publics := p.RosterServicePublics()

	r, err := makeAggregateResponse(p.suite, publics, responses)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Lvlf3("Subleader %v sent its reply with mask %b", p.ServerIdentity(), r.Mask)
	return p.SendToParent(r)
}

// dispatchLeaf prepares the signature and send it to the subleader
func (p *SubBlsFtCosi) dispatchLeaf() error {
	_, ok := p.waitAnnouncement(p.Root().Children[0])
	if !ok {
		return nil // channel closed
	}

	ok = p.verificationFn(p.Msg, p.Data)
	var r interface{}
	var err error
	if ok {
		log.Lvlf3("Leaf %v signed", p.ServerIdentity())
		r, err = p.makeResponse()
		if err != nil {
			return err
		}
	} else {
		log.Lvlf3("Leaf %v refused to sign", p.ServerIdentity())
		r, err = p.makeRefusal()
		if err != nil {
			return err
		}
	}

	return p.SendToParent(r)
}

// Sign the message pack it with the mask as a response
func (p *SubBlsFtCosi) makeResponse() (*Response, error) {
	publics := p.RosterServicePublics()

	mask, err := cosi.NewMask(p.suite, publics, p.Public())
	if err != nil {
		log.Error(err)
		return nil, err
	}

	sig, err := bls.Sign(p.suite, p.Private(), p.Msg)
	if err != nil {
		return nil, err
	}

	return &Response{
		Mask:      mask.Mask(),
		Signature: sig,
	}, nil
}

func (p *SubBlsFtCosi) makeRefusal() (*Refusal, error) {
	nonce := make([]byte, 8)
	_, err := rand.Read(nonce)
	if err != nil {
		return nil, err
	}

	sig, err := bls.Sign(p.suite, p.Private(), nonce)

	return &Refusal{Signature: sig, Nonce: nonce}, err
}

func makeAggregateResponse(suite pairing.Suite, publics []kyber.Point, responses ResponseMap) (*Response, error) {
	finalMask, err := cosi.NewMask(suite.(cosi.Suite), publics, nil)
	if err != nil {
		return nil, err
	}
	finalSignature := suite.G1().Point()

	aggMask := finalMask.Mask()
	for _, res := range responses {
		if res == nil || len(res.Signature) == 0 {
			continue
		}

		sig, err := res.Signature.Point(suite)
		if err != nil {
			return nil, err
		}
		finalSignature = finalSignature.Add(finalSignature, sig)

		aggMask, err = cosi.AggregateMasks(aggMask, res.Mask)
		if err != nil {
			return nil, err
		}
	}

	err = finalMask.SetMask(aggMask)
	if err != nil {
		return nil, err
	}

	sig, err := finalSignature.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return &Response{Signature: sig, Mask: finalMask.Mask()}, nil
}

// HandleStop is called when a Stop message is send to this node.
// It broadcasts the message to all the nodes in tree and each node will stop
// the protocol by calling p.Done.
func (p *SubBlsFtCosi) HandleStop(stop StructStop) error {
	if !stop.TreeNode.Equal(p.Root()) {
		log.Warn(p.ServerIdentity(), "received a Stop from node", stop.ServerIdentity,
			"that is not the root, ignored")
	}
	log.Lvl3("Received stop", p.ServerIdentity())

	return p.Shutdown()
}

// Shutdown closes the different channel to stop the current work
func (p *SubBlsFtCosi) Shutdown() error {
	p.stoppedOnce.Do(func() {
		log.Lvlf2("Subprotocol shut down on %v", p.ServerIdentity())
		close(p.ChannelAnnouncement)
		close(p.ChannelResponse)
		close(p.ChannelRefusal)
	})
	return nil
}

// Start is done only by root and starts the subprotocol
func (p *SubBlsFtCosi) Start() error {
	log.Lvl3(p.ServerIdentity(), "Starting subCoSi")
	if err := p.checkIntegrity(); err != nil {
		p.startChan <- false
		p.Done()
		return err
	}

	p.startChan <- true
	return nil
}

func (p *SubBlsFtCosi) checkIntegrity() error {
	if p.Msg == nil {
		return errors.New("subprotocol does not have a proposal msg")
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
	if p.Threshold < 1 {
		return fmt.Errorf("threshold of %d smaller than one node", p.Threshold)
	}

	return nil
}
