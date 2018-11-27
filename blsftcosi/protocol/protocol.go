package protocol

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/kyber/sign/bls"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

// VerificationFn is called on every node. Where msg is the message that is
// co-signed and the data is additional data for verification.
type VerificationFn func(msg []byte) bool

// init is done at startup. It defines every messages that is handled by the network
// and registers the protocols.
func init() {
	GlobalRegisterDefaultProtocols()
}

// BlsFtCosi holds the parameters of the protocol.
// It also defines a channel that will receive the final signature.
// This protocol should only exist on the root node.
type BlsFtCosi struct {
	*onet.TreeNodeInstance
	Msg            []byte
	CreateProtocol CreateProtocolFunction
	// Timeout is not a global timeout for the protocol, but a timeout used
	// for waiting for responses for sub protocols.
	Timeout        time.Duration
	Threshold      int
	FinalSignature chan BlsSignature // final signature that is sent back to client

	stoppedOnce     sync.Once
	subProtocols    []*SubBlsFtCosi
	startChan       chan bool
	subProtocolName string
	verificationFn  VerificationFn
	suite           pairing.Suite
	subTrees        BlsProtocolTree
}

// CreateProtocolFunction is a function type which creates a new protocol
// used in FtCosi protocol for creating sub leader protocols.
type CreateProtocolFunction func(name string, t *onet.Tree) (onet.ProtocolInstance, error)

// NewDefaultProtocol is the default protocol function used for registration
// with an always-true verification.
// Called by GlobalRegisterDefaultProtocols
func NewDefaultProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a []byte) bool { return true }
	return NewBlsFtCosi(n, vf, DefaultSubProtocolName, bn256.NewSuiteG2())
}

// GlobalRegisterDefaultProtocols is used to register the protocols before use,
// most likely in an init function.
func GlobalRegisterDefaultProtocols() {
	onet.GlobalProtocolRegister(DefaultProtocolName, NewDefaultProtocol)
	onet.GlobalProtocolRegister(DefaultSubProtocolName, NewDefaultSubProtocol)
}

// NewBlsFtCosi method is used to define the ftcosi protocol.
func NewBlsFtCosi(n *onet.TreeNodeInstance, vf VerificationFn, subProtocolName string, suite pairing.Suite) (onet.ProtocolInstance, error) {
	c := &BlsFtCosi{
		TreeNodeInstance: n,
		FinalSignature:   make(chan BlsSignature, 1),
		startChan:        make(chan bool, 1),
		verificationFn:   vf,
		subProtocolName:  subProtocolName,
		suite:            suite,
	}

	if len(n.Roster().List) > 1 {
		c.SetNbrSubTree(1)
	}

	return c, nil
}

// SetNbrSubTree generatesN new subtrees that will be used
// for the protocol
func (p *BlsFtCosi) SetNbrSubTree(nbr int) error {
	if nbr > len(p.Roster().List)-1 {
		return errors.New("Cannot have more subtrees than nodes")
	}
	if p.Threshold == 1 || nbr <= 0 {
		p.subTrees = []*onet.Tree{}
		return nil
	}

	var err error
	p.subTrees, err = NewBlsProtocolTree(p.Tree(), nbr)
	if err != nil {
		p.FinalSignature <- nil
		return fmt.Errorf("error in tree generation: %s", err)
	}

	return nil
}

// Shutdown stops the protocol
func (p *BlsFtCosi) Shutdown() error {
	p.stoppedOnce.Do(func() {
		for _, subFtCosi := range p.subProtocols {
			subFtCosi.HandleStop(StructStop{subFtCosi.TreeNode(), Stop{}})
		}
		close(p.startChan)
		close(p.FinalSignature)
	})
	return nil
}

// Dispatch is the main method of the protocol, defining the root node behaviour
// and sequential handling of subprotocols.
func (p *BlsFtCosi) Dispatch() error {
	defer p.Done()
	if !p.IsRoot() {
		return nil
	}

	select {
	case _, ok := <-p.startChan:
		if !ok {
			return errors.New("protocol finished prematurely")
		}
	case <-time.After(time.Second):
		return fmt.Errorf("timeout, did you forget to call Start?")
	}

	log.Lvl3("root protocol started")

	// Verification of the data
	verifyChan := make(chan bool, 1)
	go func() {
		log.Lvl3(p.ServerIdentity().Address, "starting verification")
		verifyChan <- p.verificationFn(p.Msg)
	}()

	// start all subprotocols
	p.subProtocols = make([]*SubBlsFtCosi, len(p.subTrees))
	for i, tree := range p.subTrees {
		log.Lvl2("Invoking start sub protocol", tree)
		var err error
		p.subProtocols[i], err = p.startSubProtocol(tree)
		if err != nil {
			log.Error(err)
			p.FinalSignature <- nil
			return err
		}
	}
	log.Lvl3(p.ServerIdentity().Address, "all protocols started")

	// Wait and collect all the signature responses
	responses, runningSubProtocols, err := p.collectSignatures()
	if err != nil {
		return err
	}
	log.Lvl3(p.ServerIdentity().Address, "collected all signature responses")

	_ = runningSubProtocols

	// verifies the proposal
	var verificationOk bool
	select {
	case verificationOk = <-verifyChan:
		close(verifyChan)
	case <-time.After(p.Timeout):
		log.Error(p.ServerIdentity(), "timeout while waiting for the verification!")
	}
	if !verificationOk {
		// root should not fail the verification otherwise it would not have started the protocol
		p.FinalSignature <- nil
		return fmt.Errorf("verification failed on root node")
	}

	// generate root signature
	signaturePoint, finalMask, err := generateSignature(p.suite, p.TreeNodeInstance, p.Roster().Publics(), p.Private(), responses, p.Msg, verificationOk)
	if err != nil {
		p.FinalSignature <- nil
		return err
	}

	signature, err := signaturePoint.MarshalBinary()
	if err != nil {
		p.FinalSignature <- nil
		return err
	}

	finalSignature := append(signature, finalMask.Mask()...)

	log.Lvl3(p.ServerIdentity().Address, "Created final signature", signature, finalMask, finalSignature)

	p.FinalSignature <- finalSignature

	log.Lvl3("Root-node is done without errors")
	return nil

}

// Start is done only by root and starts the protocol.
// It also verifies that the protocol has been correctly parameterized.
func (p *BlsFtCosi) Start() error {
	if p.Msg == nil {
		p.Shutdown()
		return fmt.Errorf("no proposal msg specified")
	}
	if p.CreateProtocol == nil {
		p.Shutdown()
		return fmt.Errorf("no create protocol function specified")
	}
	if p.verificationFn == nil {
		p.Shutdown()
		return fmt.Errorf("verification function cannot be nil")
	}
	if p.subProtocolName == "" {
		p.Shutdown()
		return fmt.Errorf("sub-protocol name cannot be empty")
	}
	if p.Timeout < 10*time.Nanosecond {
		p.Shutdown()
		return fmt.Errorf("unrealistic timeout")
	}
	if p.Threshold > p.Tree().Size() {
		p.Shutdown()
		return fmt.Errorf("threshold (%d) bigger than number of nodes (%d)", p.Threshold, p.Tree().Size())
	}
	if p.Threshold < 1 {
		p.Shutdown()
		return fmt.Errorf("threshold of %d smaller than one node", p.Threshold)
	}

	log.Lvl3("Starting CoSi")
	p.startChan <- true
	return nil
}

// startSubProtocol creates, parametrize and starts a subprotocol on a given tree
// and returns the started protocol.
func (p *BlsFtCosi) startSubProtocol(tree *onet.Tree) (*SubBlsFtCosi, error) {

	pi, err := p.CreateProtocol(p.subProtocolName, tree)
	if err != nil {
		return nil, err
	}
	cosiSubProtocol := pi.(*SubBlsFtCosi)
	cosiSubProtocol.Msg = p.Msg
	cosiSubProtocol.Timeout = p.Timeout / 2

	//the Threshold (minus root node) is divided evenly among the subtrees
	subThreshold := int(math.Ceil(float64(p.Threshold-1) / float64(len(p.subTrees))))
	if subThreshold > tree.Size()-1 {
		subThreshold = tree.Size() - 1
	}

	cosiSubProtocol.Threshold = subThreshold

	log.Lvl2("Starting sub protocol on", tree)
	err = cosiSubProtocol.Start()
	if err != nil {
		return nil, err
	}

	return cosiSubProtocol, err
}

// Sign the message with this node and aggregates with all child signatures (in structResponses)
// Also aggregates the child bitmasks
func generateSignature(ps pairing.Suite, t *onet.TreeNodeInstance, publics []kyber.Point, private kyber.Scalar, structResponses []StructResponse,
	msg []byte, ok bool) (kyber.Point, *cosi.Mask, error) {

	if t == nil {
		return nil, nil, fmt.Errorf("TreeNodeInstance should not be nil, but is")
	} else if structResponses == nil {
		return nil, nil, fmt.Errorf("StructResponse should not be nil, but is")
	} else if publics == nil {
		return nil, nil, fmt.Errorf("publics should not be nil, but is")
	} else if msg == nil {
		return nil, nil, fmt.Errorf("msg should not be nil, but is")
	}

	// extract lists of responses
	var signatures []kyber.Point
	var masks [][]byte

	for _, r := range structResponses {
		atmp, err := r.Signature.Point(ps)
		_ = err
		signatures = append(signatures, atmp)
		masks = append(masks, r.Mask)
	}

	//generate personal mask
	var personalMask *cosi.Mask
	if ok {
		personalMask, _ = cosi.NewMask(ps.(cosi.Suite), publics, t.Public())
	} else {
		personalMask, _ = cosi.NewMask(ps.(cosi.Suite), publics, nil)
	}

	masks = append(masks, personalMask.Mask())

	// generate personal signature and append to other sigs
	personalSig, err := bls.Sign(ps, private, msg)

	if err != nil {
		return nil, nil, err
	}
	personalPointSig, err := BlsSignature(personalSig).Point(ps)
	if !ok {
		personalPointSig = ps.G1().Point()
	}

	signatures = append(signatures, personalPointSig)

	// Aggregate all signatures
	aggSignature, aggMask, err := aggregateSignatures(ps, signatures, masks)
	if err != nil {
		log.Lvl3(t.ServerIdentity().Address, "failed to create aggregate signature")
		return nil, nil, err
	}

	//create final aggregated mask
	finalMask, err := cosi.NewMask(ps.(cosi.Suite), publics, nil)
	if err != nil {
		return nil, nil, err
	}
	err = finalMask.SetMask(aggMask)
	if err != nil {
		return nil, nil, err
	}

	log.Lvl3(t.ServerIdentity().Address, "is done aggregating signatures with total of", len(signatures), "signatures")

	return aggSignature, finalMask, nil
}
