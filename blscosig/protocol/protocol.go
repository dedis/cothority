// Package protocol implements the BLS protocol using a main protocol and multiple
// subprotocols, one for each substree.
package protocol

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/bdn"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

const defaultTimeout = 5 * time.Second

// init is done at startup. It defines every messages that is handled by the network
// and registers the protocols.
func init() {
	GlobalRegisterDefaultProtocols()
}

// VerificationFn is called on every node. Where msg is the message that is
// co-signed and the data is additional data for verification.
type VerificationFn func(msg, data []byte) bool

// CreateProtocolFunction is a function type which creates a new protocol
// used in BlsCosi protocol for creating sub leader protocols.
type CreateProtocolFunction func(name string, t *onet.Tree) (onet.ProtocolInstance, error)

// BlsCosi holds the parameters of the protocol.
// It also defines a channel that will receive the final signature.
// This protocol exists on all nodes.
type BlsCosi struct {
	*onet.TreeNodeInstance
	Msg            []byte
	Data           []byte
	Timeout        time.Duration
	Threshold      int
	FinalSignature chan BlsSignature // final signature that is sent back to client

	stoppedOnce    sync.Once
	startedOnce    sync.Once
	startChan      chan bool
	stopChan       chan bool
	verificationFn VerificationFn
	suite          *pairing.SuiteBn256
	Params         Parameters

	responses Responses
	random    *rand.Rand
}

// NewDefaultProtocol is the default protocol function used for registration
// with an always-true verification.
// Called by GlobalRegisterDefaultProtocols
func NewDefaultProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewBlsCosi(n, vf, pairing.NewSuiteBn256())
}

// GlobalRegisterDefaultProtocols is used to register the protocols before use,
// most likely in an init function.
func GlobalRegisterDefaultProtocols() {
	onet.GlobalProtocolRegister(DefaultProtocolName, NewDefaultProtocol)
}

// DefaultThreshold computes the minimal threshold authorized using
// the formula 3f+1
func DefaultThreshold(n int) int {
	f := (n - 1) / 3
	return n - f
}

// NewBlsCosi method is used to define the blscosi protocol.
func NewBlsCosi(n *onet.TreeNodeInstance, vf VerificationFn, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	nNodes := len(n.Roster().List)
	c := &BlsCosi{
		TreeNodeInstance: n,
		FinalSignature:   make(chan BlsSignature, 1),
		Timeout:          defaultTimeout,
		Params:           DefaultParams(nNodes),
		startChan:        make(chan bool, 1),
		stopChan:         make(chan bool, 1),
		verificationFn:   vf,
		suite:            suite,
		random:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	err := c.RegisterHandlers(c.handleRumor, c.handleShutdown)
	if err != nil {
		return nil, errors.New("couldn't register handlers: " + err.Error())
	}

	return c, nil
}

// Shutdown stops the protocol
func (p *BlsCosi) Shutdown() error {
	log.Lvl1("shutting down")
	p.stoppedOnce.Do(func() {
		close(p.FinalSignature)
		close(p.startChan)
		close(p.stopChan)
	})

	return nil
}

// Start is done only by root and starts the protocol.
// It also verifies that the protocol has been correctly parameterized.
func (p *BlsCosi) Start() error {
	err := p.checkIntegrity()
	if err != nil {
		p.Done()
		return err
	}

	p.startChan <- true

	return nil
}

// Dispatch is the main method of the protocol for all nodes.
func (p *BlsCosi) Dispatch() error {
	timeout := time.After(defaultTimeout)

	if p.IsRoot() {
		select {
		case _, ok := <-p.startChan:
			if !ok {
				p.Done()
				return errors.New("not started")
			}
		case <-time.After(1 * time.Second):
			p.Done()
			return errors.New("start timeout")
		}

		err := p.createResponses(p.Params.TreeMode)
		if err != nil {
			return err
		}

		ticker := time.NewTicker(p.Params.GossipTick)
		// Send the first round of rumors.
		p.sendRumors()

		// ... then one every tick until we got a signature.
		for {
			select {
			case <-ticker.C:
				p.sendRumors()
			case <-timeout:
				p.Done()
				return errors.New("gossip protocol timeout")
			case <-p.stopChan:
				return nil
			}
		}
	} else {
		// Non-root node needs to shutdown if no shutdown message arrives.
		select {
		case <-p.stopChan:
		case <-timeout:
			p.Done()
			return errors.New("timeout")
		}
	}

	return nil
}

func (p *BlsCosi) handleRumor(msg RumorMessage) error {
	select {
	case <-p.stopChan:
		// Protocol has stopped so we ignore incoming rumors.
		return nil
	default:
	}

	if !p.IsRoot() {
		p.startedOnce.Do(func() {
			p.initGossip(&msg.Rumor)
		})
	}

	oldCount := p.responses.Count()

	if err := p.responses.Update(msg.ResponseMap); err != nil {
		return err
	}

	if p.IsRoot() {
		if p.isEnough() {
			shutdown, err := p.finalize()
			if err != nil {
				return err
			}

			p.SendToChildrenInParallel(shutdown)
			p.Done()
		}
	} else {
		if oldCount < p.responses.Count() {
			// Only spread new information.
			p.sendRumors()
		}
	}

	return nil
}

func (p *BlsCosi) handleShutdown(msg ShutdownMessage) error {
	select {
	case <-p.stopChan:
		return nil
	default:
	}

	if err := p.verifyShutdown(msg); err != nil {
		return errors.New("couldn't verify the shutdown message: " + err.Error())
	}

	errs := p.SendToChildrenInParallel(&msg.Shutdown)
	if len(errs) > 0 {
		log.Error("Couldn't send the shutdown message to all children: ", errs)
	}

	p.Done()

	return nil
}

func (p *BlsCosi) createResponses(treeMode bool) (err error) {
	if treeMode {
		p.responses, err = NewTreeResponses(p.suite, p.Publics())
		if err != nil {
			return
		}
	} else {
		p.responses = make(SimpleResponses)
	}

	// Add own signature.
	return p.trySign()
}

func (p *BlsCosi) initGossip(r *Rumor) error {
	p.Params = r.Params
	p.Msg = r.Msg
	p.Data = r.Data

	return p.createResponses(p.Params.TreeMode)
}

func (p *BlsCosi) finalize() (*Shutdown, error) {
	log.Lvl3(p.ServerIdentity().Address, "collected all signature responses", p.responses.Count())
	// generate root signature
	signaturePoint, finalMask, err := p.responses.Aggregate(p.suite, p.Publics())
	if err != nil {
		return nil, err
	}

	signature, err := signaturePoint.MarshalBinary()
	if err != nil {
		return nil, err
	}

	finalSig := append(signature, finalMask.Mask()...)
	log.Lvlf3("%v created final signature %x with mask %b", p.ServerIdentity(), signature, finalMask.Mask())
	p.FinalSignature <- finalSig

	// Sign shutdown message
	rootSig, err := bdn.Sign(p.suite, p.Private(), finalSig)
	if err != nil {
		return nil, err
	}

	return &Shutdown{finalSig, rootSig}, nil
}

func (p *BlsCosi) trySign() error {
	if !p.verificationFn(p.Msg, p.Data) {
		log.Lvlf4("Node %v refused to sign", p.ServerIdentity())
		return nil
	}
	own, idx, err := p.makeResponse()
	if err != nil {
		return err
	}
	p.responses.Add(idx, own)
	log.Lvlf4("Node %v signed", p.ServerIdentity())
	return nil
}

// sendRumors sends a rumor message to some peers.
func (p *BlsCosi) sendRumors() {
	targets, err := p.getRandomPeers(p.Params.RumorPeers)
	if err != nil {
		log.Error("Couldn't get random peers:", err)
		return
	}
	for _, target := range targets {
		p.sendRumor(target)
	}
}

// sendRumor sends the given signatures to a random peer.
func (p *BlsCosi) sendRumor(target *onet.TreeNode) {
	err := p.SendTo(target, &Rumor{p.Params, p.responses.Map(), p.Msg, p.Data})
	if err != nil {
		log.Lvl3(err)
	}
}

// verifyShutdown verifies the legitimacy of a shutdown message.
func (p *BlsCosi) verifyShutdown(msg ShutdownMessage) error {
	publics := p.Publics()
	rootPublic := publics[p.Root().RosterIndex]

	// verify final signature
	err := msg.FinalCoSignature.VerifyAggregate(p.suite, p.Msg, publics)
	if err != nil {
		return err
	}

	// verify root signature of final signature
	return verify(p.suite, msg.RootSig, msg.FinalCoSignature, rootPublic)
}

// verify checks the signature over the message with a single key
func verify(suite pairing.Suite, sig []byte, msg []byte, public kyber.Point) error {
	if len(msg) == 0 {
		return errors.New("no message provided to Verify()")
	}
	if len(sig) == 0 {
		return errors.New("no signature provided to Verify()")
	}
	err := bdn.Verify(suite, public, msg, sig)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}
	return nil
}

// isEnough returns true if we have enough responses.
func (p *BlsCosi) isEnough() bool {
	return p.responses.Count() >= p.Threshold
}

// getRandomPeers returns a slice of random peers (not including self).
func (p *BlsCosi) getRandomPeers(numTargets int) ([]*onet.TreeNode, error) {
	t := p.List()
	nodes := make([]*onet.TreeNode, len(t))
	copy(nodes, t)

	// If there is only one node, that means the root is alone, so it needs
	// to talk to itself to operate the gossip protocol.
	if len(nodes) > 1 {
		idx := 0
		for i, n := range nodes {
			if n.Equal(p.TreeNode()) {
				idx = i
			}
		}

		nodes = append(nodes[:idx], nodes[idx+1:]...)
	}

	if len(nodes) < numTargets {
		return nil, errors.New("not enough nodes in the roster")
	}

	p.random.Shuffle(len(nodes), func(i, j int) {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	})

	return nodes[:numTargets], nil
}

// checkIntegrity checks if the protocol has been instantiated with
// correct parameters
func (p *BlsCosi) checkIntegrity() error {
	if p.Msg == nil {
		return fmt.Errorf("no proposal msg specified")
	}
	if p.verificationFn == nil {
		return fmt.Errorf("verification function cannot be nil")
	}
	if p.Timeout < 500*time.Microsecond {
		return fmt.Errorf("unrealistic timeout")
	}
	if p.Threshold > p.Tree().Size() {
		return fmt.Errorf("threshold (%d) bigger than number of nodes (%d)", p.Threshold, p.Tree().Size())
	}
	if p.Threshold < 1 {
		return fmt.Errorf("threshold of %d smaller than one node", p.Threshold)
	}

	return nil
}

// checkFailureThreshold returns true when the number of failures
// is above the threshold
func (p *BlsCosi) checkFailureThreshold(numFailure int) bool {
	return numFailure > len(p.Roster().List)-p.Threshold
}

// Sign the message and pack it with the mask as a response
// idx is this node's index
func (p *BlsCosi) makeResponse() (*Response, int, error) {
	mask, err := sign.NewMask(p.suite, p.Publics(), p.Public())
	if err != nil {
		return nil, 0, err
	}

	idx := mask.IndexOfNthEnabled(0) // The only set bit is this node's
	if idx < 0 {
		return nil, 0, errors.New("Couldn't find own index")
	}

	sig, err := bdn.Sign(p.suite, p.Private(), p.Msg)
	if err != nil {
		return nil, 0, err
	}

	return &Response{
		Mask:      mask.Mask(),
		Signature: sig,
	}, idx, nil
}
