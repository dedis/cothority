// Package protocol is the fault tolerant cosi protocol implementation.
//
// For more information on the protocol, please see
// https://github.com/dedis/cothority/blob/master/ftcosi/protocol/README.md.
package protocol

import (
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"math"
)

// VerificationFn is called on every node. Where msg is the message that is
// co-signed and the data is additional data for verification.
type VerificationFn func(msg []byte, data []byte) bool

// init is done at startup. It defines every messages that is handled by the network
// and registers the protocols.
func init() {
	network.RegisterMessages(Announcement{}, Commitment{}, Challenge{}, Response{}, Stop{})
}

// FtCosi holds the parameters of the protocol.
// It also defines a channel that will receive the final signature.
// This protocol should only exist on the root node.
type FtCosi struct {
	*onet.TreeNodeInstance

	NSubtrees      int
	Msg            []byte
	Data           []byte
	CreateProtocol CreateProtocolFunction
	// Timeout is not a global timeout for the protocol, but a timeout used
	// for waiting for responses for sub protocols.
	Timeout        time.Duration
	Threshold      int
	FinalSignature chan []byte

	publics         []kyber.Point
	stoppedOnce     sync.Once
	startChan       chan bool
	subProtocolName string
	verificationFn  VerificationFn
	suite           cosi.Suite
}

// CreateProtocolFunction is a function type which creates a new protocol
// used in FtCosi protocol for creating sub leader protocols.
type CreateProtocolFunction func(name string, t *onet.Tree) (onet.ProtocolInstance, error)

// NewDefaultProtocol is the default protocol function used for registration
// with an always-true verification.
func NewDefaultProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewFtCosi(n, vf, DefaultSubProtocolName, cothority.Suite)
}

// GlobalRegisterDefaultProtocols is used to register the protocols before use,
// most likely in an init function.
func GlobalRegisterDefaultProtocols() {
	onet.GlobalProtocolRegister(DefaultProtocolName, NewDefaultProtocol)
	onet.GlobalProtocolRegister(DefaultSubProtocolName, NewDefaultSubProtocol)
}

// NewFtCosi method is used to define the ftcosi protocol.
func NewFtCosi(n *onet.TreeNodeInstance, vf VerificationFn, subProtocolName string, suite cosi.Suite) (onet.ProtocolInstance, error) {

	c := &FtCosi{
		TreeNodeInstance: n,
		FinalSignature:   make(chan []byte, 1),
		Data:             make([]byte, 0),
		publics:          n.Roster().Publics(),
		startChan:        make(chan bool, 1),
		verificationFn:   vf,
		subProtocolName:  subProtocolName,
		suite:            suite,
	}

	return c, nil
}

// Shutdown stops the protocol
func (p *FtCosi) Shutdown() error {
	p.stoppedOnce.Do(func() {
		close(p.FinalSignature)
	})
	return nil
}

// Dispatch is the main method of the protocol, defining the root node behaviour
// and sequential handling of subprotocols.
func (p *FtCosi) Dispatch() error { //TODO R: verify that it sends empty signature
	defer p.Done()
	if !p.IsRoot() {
		return nil
	}

	select {
	case _, ok := <-p.startChan:
		if !ok {
			log.Lvl1("protocol finished prematurely")
			return nil
		}
		close(p.startChan)
	case <-time.After(time.Second):
		return fmt.Errorf("timeout, did you forget to call Start?")
	}

	log.Lvl3("root protocol started")

	verifyChan := make(chan bool, 1)
	go func() {
		log.Lvl3(p.ServerIdentity().Address, "starting verification")
		verifyChan <- p.verificationFn(p.Msg, p.Data)
	}()

	// generate trees
	nNodes := p.Tree().Size()
	trees, err := genTrees(p.Tree().Roster, nNodes, p.NSubtrees)
	if err != nil {
		return fmt.Errorf("error in tree generation: %s", err)
	}

	// if one node or threshold is one, sign without subprotocols
	if nNodes == 1 || p.Threshold == 1 {
		trees = make([]*onet.Tree, 0)
	}

	// start all subprotocols
	cosiSubProtocols := make([]*SubFtCosi, len(trees))
	for i, tree := range trees {
		cosiSubProtocols[i], err = p.startSubProtocol(tree)
		if err != nil {
			return err
		}
	}
	log.Lvl3(p.ServerIdentity().Address, "all protocols started")

	// collect commitments
	commitments, runningSubProtocols, err := p.collectCommitments(trees, cosiSubProtocols)
	if err != nil {
		return err
	}

	var secret kyber.Scalar
	// verifies the proposal
	verificationOk := <-verifyChan
	close(verifyChan)
	if verificationOk {
		//add own commitment
		var personalCommitment kyber.Point
		secret, personalCommitment = cosi.Commit(p.suite)
		personalMask, err := cosi.NewMask(p.suite, p.publics, p.Public())
		if err != nil {
			return err
		}
		personalStructCommitment := StructCommitment{p.TreeNode(),
			Commitment{personalCommitment, personalMask.Mask(), 0}}
		commitments = append(commitments, personalStructCommitment)
	} else {
		// root should not fail the verification otherwise it would not have started the protocol
		p.FinalSignature <- nil
		return fmt.Errorf("verification failed on root node")
	}

	// generate own aggregated commitment
	commitment, finalMask, err := aggregateCommitments(p.suite, p.publics, commitments)
	if err != nil {
		return err
	}

	log.Lvl3("root-node generating global challenge")
	cosiChallenge, err := cosi.Challenge(p.suite, commitment, finalMask.AggregatePublic, p.Msg)
	if err != nil {
		return err
	}
	structChallenge := StructChallenge{p.TreeNode(), Challenge{cosiChallenge}}

	// send challenge to every subprotocol
	for _, coSiProtocol := range runningSubProtocols {
		subProtocol := coSiProtocol
		subProtocol.ChannelChallenge <- structChallenge
	}

	// get response from all subprotocols
	responses := make([]StructResponse, 0)
	errChan := make(chan error, len(runningSubProtocols))
	var responsesMut sync.Mutex
	var responsesWg sync.WaitGroup
	for i, cosiSubProtocol := range runningSubProtocols {
		responsesWg.Add(1)
		go func(i int, subProto *SubFtCosi) {
			defer responsesWg.Done()
			select {
			case response := <-subProto.subResponse:
				responsesMut.Lock()
				responses = append(responses, response)
				responsesMut.Unlock()
			case <-time.After(p.Timeout):
				// This should never happen, as the subProto should return before that
				// timeout, even if it didn't receive enough responses.
				errChan <- fmt.Errorf("timeout should not happen while waiting for response: %d", i)
			}
		}(i, cosiSubProtocol)
	}
	responsesWg.Wait()

	// check errors if any
	close(errChan)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("nodes timed out while waiting for response %v", errs)
	}

	// generate own response
	personalResponse, err := cosi.Response(p.suite, p.Private(), secret, structChallenge.CoSiChallenge)
	if err != nil {
		return fmt.Errorf("error while generating own response: %s", err)
	}
	responses = append(responses, StructResponse{p.TreeNode(), Response{personalResponse}})

	aggResponse, err := aggregateResponses(p.suite, responses)
	if err != nil {
		return err
	}

	//starts final signature
	log.Lvl3(p.ServerIdentity().Address, "starts final signature")
	var signature []byte
	signature, err = cosi.Sign(p.suite, commitment, aggResponse, finalMask)
	if err != nil {
		return err
	}
	p.FinalSignature <- signature

	log.Lvl3("Root-node is done without errors")
	return nil
}

// Start is done only by root and starts the protocol.
// It also verifies that the protocol has been correctly parameterized.
func (p *FtCosi) Start() error {
	if p.Msg == nil {
		close(p.startChan)
		return fmt.Errorf("no proposal msg specified")
	}
	if p.CreateProtocol == nil {
		close(p.startChan)
		return fmt.Errorf("no create protocol function specified")
	}
	if p.verificationFn == nil {
		close(p.startChan)
		return fmt.Errorf("verification function cannot be nil")
	}
	if p.subProtocolName == "" {
		close(p.startChan)
		return fmt.Errorf("sub-protocol name cannot be empty")
	}
	if p.Timeout < 10*time.Nanosecond {
		close(p.startChan)
		return fmt.Errorf("unrealistic timeout")
	}
	if p.Threshold > len(p.publics) {
		close(p.startChan)
		return fmt.Errorf("threshold bigger than number of nodes")
	}

	if p.NSubtrees < 1 {
		log.Warn("no number of subtree specified, using one subtree")
		p.NSubtrees = 1
	}
	if p.Threshold == 0 {
		log.Lvl3("no threshold specified, using \"as much as possible\" policy")
	}

	log.Lvl3("Starting CoSi")
	p.startChan <- true
	return nil
}

// startSubProtocol creates, parametrize and starts a subprotocol on a given tree
// and returns the started protocol.
func (p *FtCosi) startSubProtocol(tree *onet.Tree) (*SubFtCosi, error) {

	pi, err := p.CreateProtocol(p.subProtocolName, tree)
	if err != nil {
		return nil, err
	}

	cosiSubProtocol := pi.(*SubFtCosi)
	cosiSubProtocol.Publics = p.publics
	cosiSubProtocol.Msg = p.Msg
	cosiSubProtocol.Data = p.Data
	// We allow for one subleader failure during the commit phase, and thus
	// only allocate one third of the ftcosi budget to the subprotocol.
	cosiSubProtocol.Timeout = p.Timeout / 3

	//the Threshold (minus root node) is divided evenly among the subtrees
	subThreshold := int(math.Ceil(float64(p.Threshold-1) / float64(p.NSubtrees)))
	if subThreshold > tree.Size()-1 {
		subThreshold = tree.Size() - 1
	}

	cosiSubProtocol.Threshold = subThreshold

	err = cosiSubProtocol.Start()
	if err != nil {
		return nil, err
	}

	return cosiSubProtocol, err
}
