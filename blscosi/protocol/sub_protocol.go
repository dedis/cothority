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

// SubBlsCosi holds the different channels used to receive the different protocol messages.
type SubBlsCosi struct {
	*onet.TreeNodeInstance
	Msg            []byte
	Data           []byte
	Timeout        time.Duration
	Threshold      int
	stoppedOnce    sync.Once
	verificationFn VerificationFn
	suite          *pairing.SuiteBn256
	startChan      chan bool
	closeChan      chan struct{}

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
	return NewSubBlsCosi(n, vf, pairing.NewSuiteBn256())
}

// NewSubBlsCosi is used to define the subprotocol and to register
// the channels where the messages will be received.
func NewSubBlsCosi(n *onet.TreeNodeInstance, vf VerificationFn, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	// tests if it's a three level tree
	moreThreeLevel := false
	n.Tree().Root.Visit(0, func(depth int, n *onet.TreeNode) {
		if depth > 2 {
			moreThreeLevel = true
		}
	})
	if moreThreeLevel {
		return nil, fmt.Errorf("subBlsCosi launched with a more than three level tree")
	}

	c := &SubBlsCosi{
		TreeNodeInstance: n,
		verificationFn:   vf,
		suite:            suite,
		startChan:        make(chan bool, 1),
		closeChan:        make(chan struct{}),
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
func (p *SubBlsCosi) Dispatch() error {
	defer p.Done()

	// Send announcement to start sending signatures
	if p.IsRoot() {
		return p.dispatchRoot()
	} else if p.Parent().Equal(p.Root()) {
		return p.dispatchSubLeader()
	}

	return p.dispatchLeaf()
}

// HandleStop is called when a Stop message is send to this node.
// It broadcasts the message to all the nodes in tree and each node will stop
// the protocol by calling p.Done.
func (p *SubBlsCosi) HandleStop(stop StructStop) error {
	if !stop.TreeNode.Equal(p.Root()) {
		log.Warn(p.ServerIdentity(), "received a Stop from node", stop.ServerIdentity,
			"that is not the root, ignored")
	}
	log.Lvl3("Received stop", p.ServerIdentity())

	return p.Shutdown()
}

// Shutdown closes the different channel to stop the current work
func (p *SubBlsCosi) Shutdown() error {
	p.stoppedOnce.Do(func() {
		log.Lvlf3("Subprotocol shut down on %v", p.ServerIdentity())
		// Only this channel is closed to cut off expensive operations
		// and select statements but we let other channels be cleaned
		// by the GC to avoid sending to closed channel
		close(p.startChan)
		close(p.closeChan)
	})
	return nil
}

// Start is done only by root and starts the subprotocol
func (p *SubBlsCosi) Start() error {
	log.Lvl3(p.ServerIdentity(), "Starting subCoSi")
	if err := p.checkIntegrity(); err != nil {
		p.startChan <- false
		p.Done()
		return err
	}

	p.startChan <- true
	return nil
}

// waitAnnouncement waits for an announcement of the right node
func (p *SubBlsCosi) waitAnnouncement(parent *onet.TreeNode) *Announcement {
	var a *Announcement
	// Keep looping until the correct announcement to prevent
	// an attacker from killing the protocol with false message
	for a == nil {
		select {
		case <-p.closeChan:
			return nil
		case msg := <-p.ChannelAnnouncement:
			if parent.Equal(msg.TreeNode) {
				a = &msg.Announcement
			}
		}
	}

	p.Msg = a.Msg
	p.Data = a.Data
	p.Timeout = a.Timeout
	p.Threshold = a.Threshold

	return a
}

// dispatchRoot takes care of sending announcements to the children and
// waits for the response with the signatures of the children
func (p *SubBlsCosi) dispatchRoot() error {
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
	case <-p.closeChan:
		return nil
	case reply := <-p.ChannelResponse:
		if reply.Equal(p.Root().Children[0]) {
			// Transfer the response to the parent protocol
			p.subResponse <- reply
		}
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
func (p *SubBlsCosi) dispatchSubLeader() error {
	a := p.waitAnnouncement(p.Root())
	if a == nil {
		return nil
	}

	errs := p.SendToChildrenInParallel(a)
	if len(errs) > 0 {
		log.Error(errs)
	}

	responses := make(ResponseMap)
	for _, c := range p.Children() {
		public := p.NodePublic(c.ServerIdentity)
		// Accept response for those identities only
		responses[public.String()] = nil
	}

	own, err := p.makeResponse()
	if ok := p.verificationFn(p.Msg, p.Data); ok {
		log.Lvlf3("Subleader %v signed", p.ServerIdentity())
		responses[p.Public().String()] = own
	}

	// we need to timeout the children faster than the root timeout to let it
	// know the subleader is alive, but some children are failing
	timeout := time.After(p.Timeout / 2)
	// If an error happens when sending the announcement, we can assume there
	// will be a timeout from this node
	done := len(errs)
	for done < len(p.Children()) {
		select {
		case <-p.closeChan:
			return nil
		case reply := <-p.ChannelResponse:
			public := searchPublicKey(p.TreeNodeInstance, reply.ServerIdentity)
			if public != nil {
				r, ok := responses[public.String()]
				if !ok {
					log.Warnf("Got a message from an unknown node %v", reply.ServerIdentity.ID)
				} else if r == nil {
					if public == nil {
						log.Warnf("Tentative to forge a server identity or unknown node.")
					} else if err := bls.Verify(p.suite, public, p.Msg, reply.Signature); err == nil {
						responses[public.String()] = &reply.Response
						done++
					}
				} else {
					log.Warnf("Duplicate message from %v", reply.ServerIdentity)
				}
			} else {
				log.Warnf("Received unknown server identity %v", reply.ServerIdentity)
			}
		case reply := <-p.ChannelRefusal:
			public := searchPublicKey(p.TreeNodeInstance, reply.ServerIdentity)
			r, ok := responses[public.String()]
			serviceName := onet.ServiceFactory.Name(p.Token().ServiceID)

			if !ok {
				log.Warnf("Got a message from an unknown node %v", reply.ServerIdentity.ID)
			} else if r == nil {
				if err := bls.Verify(p.suite, reply.ServerIdentity.ServicePublic(serviceName), reply.Nonce, reply.Signature); err == nil {
					// The child gives an empty signature as a mark of refusal
					responses[public.String()] = &Response{}
					done++
				} else {
					log.Warnf("Tentative to send a unsigned refusal from %v", reply.ServerIdentity.ID)
				}
			} else {
				log.Warnf("Duplicate refusal from %v", reply.ServerIdentity)
			}
		case <-timeout:
			log.Lvlf3("Subleader reached timeout waiting for children responses: %v", p.ServerIdentity())
			// Use whatever we received until then to try to finish
			// the protocol
			done = len(p.Children())
		}
	}

	r, err := makeAggregateResponse(p.suite, p.Publics(), responses)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Lvlf3("Subleader %v sent its reply with mask %b", p.ServerIdentity(), r.Mask)
	return p.SendToParent(r)
}

// dispatchLeaf prepares the signature and send it to the subleader
func (p *SubBlsCosi) dispatchLeaf() error {
	a := p.waitAnnouncement(p.Root().Children[0])
	if a == nil {
		return nil
	}

	res := make(chan bool)
	go p.makeVerification(res)

	// give a chance to avoid sending the response if a stop
	// has been requested
	select {
	case <-p.closeChan:
		// ...but still wait for the response so that we don't leak the goroutine
		<-res
		return nil
	case ok := <-res:
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
}

// Sign the message and pack it with the mask as a response
func (p *SubBlsCosi) makeResponse() (*Response, error) {
	mask, err := cosi.NewMask(p.suite, p.Publics(), p.Public())
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

// makeRefusal will sign a random nonce so that we can check
// that the refusal is not forged
func (p *SubBlsCosi) makeRefusal() (*Refusal, error) {
	nonce := make([]byte, 8)
	_, err := rand.Read(nonce)
	if err != nil {
		return nil, err
	}

	sig, err := bls.Sign(p.suite, p.Private(), nonce)

	return &Refusal{Signature: sig, Nonce: nonce}, err
}

// makeAggregateResponse takes all the responses from the children and the subleader to
// aggregate the signature and the mask
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

// makeVerification executes the verification function provided and
// returns the result in the given channel
func (p *SubBlsCosi) makeVerification(out chan bool) {
	out <- p.verificationFn(p.Msg, p.Data)
}

// checkIntegrity checks that the subprotocol can start with the current
// parameters
func (p *SubBlsCosi) checkIntegrity() error {
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
