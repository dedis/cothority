package protocol

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// VerificationFn is called on every node. Where msg is the message that is
// co-signed and the data is additional data for verification.
type VerificationFn func(msg []byte, data []byte) bool

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
	NSubtrees      int
	Msg            []byte
	Data           []byte
	CreateProtocol CreateProtocolFunction
	// Timeout is not a global timeout for the protocol, but a timeout used
	// for waiting for responses for sub protocols.
	Timeout        time.Duration
	Threshold      int
	FinalSignature chan []byte // final signature that is sent back to client

	stoppedOnce     sync.Once
	subProtocols    []*SubBlsFtCosi
	startChan       chan bool
	subProtocolName string
	verificationFn  VerificationFn
	suite           cosi.Suite
	pairingSuite    pairing.Suite
}

// CreateProtocolFunction is a function type which creates a new protocol
// used in FtCosi protocol for creating sub leader protocols.
type CreateProtocolFunction func(name string, t *onet.Tree) (onet.ProtocolInstance, error)

// NewDefaultProtocol is the default protocol function used for registration
// with an always-true verification.
// Called by GlobalRegisterDefaultProtocols
func NewDefaultProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewBlsFtCosi(n, vf, DefaultSubProtocolName, cothority.Suite, bn256.NewSuite())
}

// GlobalRegisterDefaultProtocols is used to register the protocols before use,
// most likely in an init function.
func GlobalRegisterDefaultProtocols() {
	onet.GlobalProtocolRegister(DefaultProtocolName, NewDefaultProtocol)
	onet.GlobalProtocolRegister(DefaultSubProtocolName, NewDefaultSubProtocol)
}

// NewBlsFtCosi method is used to define the ftcosi protocol.
func NewBlsFtCosi(n *onet.TreeNodeInstance, vf VerificationFn, subProtocolName string, suite cosi.Suite, pairingSuite pairing.Suite) (onet.ProtocolInstance, error) {

	c := &BlsFtCosi{
		TreeNodeInstance: n,
		FinalSignature:   make(chan []byte, 1),
		Data:             make([]byte, 0),
		startChan:        make(chan bool, 1),
		verificationFn:   vf,
		subProtocolName:  subProtocolName,
		suite:            suite,
		pairingSuite:     pairingSuite,
	}

	return c, nil
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
		verifyChan <- p.verificationFn(p.Msg, p.Data)
	}()

	// generate trees
	nNodes := p.Tree().Size()
	trees, err := genTrees(p.Tree().Roster, p.Tree().Root.RosterIndex, nNodes, p.NSubtrees)
	if err != nil {
		p.FinalSignature <- nil
		return fmt.Errorf("error in tree generation: %s", err)
	}

	// if one node, sign without subprotocols
	if nNodes == 1 || p.Threshold == 1 {
		trees = make([]*onet.Tree, 0)
	}

	// start all subprotocols
	p.subProtocols = make([]*SubBlsFtCosi, len(trees))
	for i, tree := range trees {
		log.Lvl2("Invoking start sub protocol", tree)
		p.subProtocols[i], err = p.startSubProtocol(tree)
		if err != nil {
			log.Error(err)
			p.FinalSignature <- nil
			return err
		}
	}
	log.Lvl3(p.ServerIdentity().Address, "all protocols started")

	// Wait and collect all the signature responses
	responses, runningSubProtocols, err := p.collectSignatures(trees, p.subProtocols, p.Roster().Publics())
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
	signaturePoint, finalMask, err := generateSignature(p.pairingSuite, p.TreeNodeInstance, p.Roster().Publics(), p.Private(), responses, p.Msg, verificationOk)
	if err != nil {
		p.FinalSignature <- nil
		return err
	}

	signature, err := signaturePoint.MarshalBinary()
	if err != nil {
		p.FinalSignature <- nil
		return err
	}

	finalSignature := append(signature, finalMask.mask...)

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

	if p.NSubtrees < 1 {
		log.Warn("no number of subtree specified, using one subtree")
		p.NSubtrees = 1
	}
	if p.NSubtrees >= p.Tree().Size() && p.NSubtrees > 1 {
		p.Shutdown()
		return fmt.Errorf("cannot create more subtrees (%d) than there are non-root nodes (%d) in the tree",
			p.NSubtrees, p.Tree().Size()-1)
	}

	log.Lvl3("Starting CoSi")
	p.startChan <- true
	return nil
}

func (p *BlsFtCosi) getLeaves() []*network.ServerIdentity {
	log.Lvlf2("%d", len(p.Children()))

	return dfsLeaves(p.TreeNode())
}

func dfsLeaves(tn *onet.TreeNode) []*network.ServerIdentity {
	if tn.IsLeaf() {
		return []*network.ServerIdentity{tn.ServerIdentity}
	}

	si := []*network.ServerIdentity{}
	for _, c := range tn.Children {
		si = append(si, dfsLeaves(c)...)
	}
	return si
}

func (p *BlsFtCosi) getSubLeaders() []*network.ServerIdentity {
	si := []*network.ServerIdentity{}

	for _, c := range p.Children() {
		if !c.IsLeaf() {
			si = append(si, c.ServerIdentity)
		}
	}

	return si
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
	cosiSubProtocol.Data = p.Data
	cosiSubProtocol.Timeout = p.Timeout / 2

	//the Threshold (minus root node) is divided evenly among the subtrees
	subThreshold := int(math.Ceil(float64(p.Threshold-1) / float64(p.NSubtrees)))
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
