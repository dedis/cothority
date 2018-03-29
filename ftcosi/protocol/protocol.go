// Package protocol is the fault tolerant cosi protocol implementation.
//
// For more information on the protocol, please see
// https://gopkg.in/dedis/cothority.v2/blob/master/ftcosi/protocol/README.md.
package protocol

import (
	"fmt"
	"sync"
	"time"

	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/kyber.v2/sign/cosi"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
	"gopkg.in/dedis/onet.v2/network"
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

	var list []kyber.Point
	for _, t := range n.Tree().List() {
		list = append(list, t.ServerIdentity.Public)
	}

	c := &FtCosi{
		TreeNodeInstance: n,
		FinalSignature:   make(chan []byte, 1),
		Data:             make([]byte, 0),
		publics:          list,
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
func (p *FtCosi) Dispatch() error {
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

	log.Lvl3("leader protocol started")

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

	// if one node, sign without subprotocols
	if nNodes == 1 {
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

	commitments, runningSubProtocols, err := p.collectCommitments(trees, cosiSubProtocols)
	if err != nil {
		return err
	}

	// generate challenge
	log.Lvl3("root-node generating global challenge")
	secret, commitment, finalMask, err := generateCommitmentAndAggregate(p.suite, p.TreeNodeInstance, p.publics, commitments)
	if err != nil {
		return err
	}

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
				errChan <- fmt.Errorf("%v", i)
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

	// signs the proposal
	if !<-verifyChan {
		// root should not fail the verification otherwise it would not have
		// started the protocol
		p.FinalSignature <- nil
		return fmt.Errorf("verification failed on root node")
	}
	response, err := generateResponse(p.suite, p.TreeNodeInstance, responses, secret, cosiChallenge)
	if err != nil {
		return err
	}
	log.Lvl3(p.ServerIdentity().Address, "starts final signature")
	var signature []byte
	signature, err = cosi.Sign(p.suite, commitment, response, finalMask)
	if err != nil {
		return err
	}
	p.FinalSignature <- signature

	log.Lvl3("Root-node is done without errors")
	return nil
}

func (p *FtCosi) collectCommitments(trees []*onet.Tree,
	cosiSubProtocols []*SubFtCosi) ([]StructCommitment, []*SubFtCosi, error) {
	// get all commitments, restart subprotocols where subleaders do not respond
	var mut sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(cosiSubProtocols))
	commitments := make([]StructCommitment, 0)
	runningSubProtocols := make([]*SubFtCosi, 0)

	for i, subProtocol := range cosiSubProtocols {
		wg.Add(1)
		go func(i int, subProtocol *SubFtCosi) {
			defer wg.Done()
			for {
				select {
				case <-subProtocol.subleaderNotResponding:
					subleaderID := trees[i].Root.Children[0].RosterIndex
					log.Lvlf2("subleader from tree %d (id %d) failed, restarting it", i, subleaderID)

					// send stop signal
					subProtocol.HandleStop(StructStop{subProtocol.TreeNode(), Stop{}})

					// generate new tree
					newSubleaderID := subleaderID + 1
					if newSubleaderID >= len(trees[i].Roster.List) {
						log.Lvl2("subprotocol", i, "failed with every subleader, ignoring this subtree")
						return
					}
					var err error
					trees[i], err = genSubtree(trees[i].Roster, newSubleaderID)
					if err != nil {
						errChan <- fmt.Errorf("(node %v) %v", i, err)
						return
					}

					// restart subprotocol
					subProtocol, err = p.startSubProtocol(trees[i])
					if err != nil {
						err = fmt.Errorf("(node %v) error in restarting of subprotocol: %s", i, err)
						errChan <- err
						return
					}
					mut.Lock()
					cosiSubProtocols[i] = subProtocol
					mut.Unlock()
				case commitment := <-subProtocol.subCommitment:
					mut.Lock()
					runningSubProtocols = append(runningSubProtocols, subProtocol)
					commitments = append(commitments, commitment)
					mut.Unlock()
					return
				case <-time.After(p.Timeout):
					err := fmt.Errorf("(node %v) didn't get commitment after timeout %v", i, p.Timeout)
					errChan <- err
					return
				}
			}
		}(i, subProtocol)
	}
	wg.Wait()

	close(errChan)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("failed to collect commitments with errors %v", errs)
	}
	return commitments, runningSubProtocols, nil
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
	if p.Timeout < 10 {
		close(p.startChan)
		return fmt.Errorf("unrealistic timeout")
	}

	if p.NSubtrees < 1 {
		p.NSubtrees = 1
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
	cosiSubProtocol.Timeout = p.Timeout / 2

	err = cosiSubProtocol.Start()
	if err != nil {
		return nil, err
	}

	return cosiSubProtocol, err
}
