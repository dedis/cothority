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

	// signs the proposal
	ok := <-verifyChan
	if !ok {
		// root should not fail the verification otherwise it would not have
		// started the protocol
		p.FinalSignature <- nil
		return fmt.Errorf("verification failed on root node")
	}

	log.Lvl3("root-node generating global challenge")
	secret, commitment, finalMask, err := generateCommitmentAndAggregate(p.suite, p.TreeNodeInstance, p.publics, commitments, ok)
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

	// generate challenge
	response, err := generateResponse(p.suite, p.TreeNodeInstance, responses, secret, cosiChallenge, ok)
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

	sharedMask, err := cosi.NewMask(p.suite, p.publics, nil)
	if err != nil {
		return nil, nil, err
	}

	errChan := make(chan error, len(cosiSubProtocols))
	thresholdReachedChan := make(chan bool, 2*len(cosiSubProtocols))    //TODO: remove
	subProtocolsCommitments := make(map[*SubFtCosi]StructCommitment, 0) //TODO: use this as a channel to do threshold check
	closingChan := make(chan bool)

	var closingWg sync.WaitGroup
	for i, subProtocol := range cosiSubProtocols {
		closingWg.Add(1)
		go func(i int, subProtocol *SubFtCosi) {
			defer closingWg.Done()
			timeout := time.After(p.Timeout / 2)
			for {
				select {
				case <-closingChan:
					return
				case <-subProtocol.subleaderNotResponding:
					subleaderID := trees[i].Root.Children[0].RosterIndex
					log.Lvlf2("(subprotocol %v) subleader with id %d failed, restarting subprotocol", i, subleaderID)

					// send stop signal
					subProtocol.HandleStop(StructStop{subProtocol.TreeNode(), Stop{}})

					// generate new tree
					newSubleaderID := subleaderID + 1
					if newSubleaderID >= len(trees[i].Roster.List) {
						log.Lvl2("(subprotocol %v) failed with every subleader, ignoring this subtree")
						return
					}
					var err error
					trees[i], err = genSubtree(trees[i].Roster, newSubleaderID)
					if err != nil {
						errChan <- fmt.Errorf("(subprotocol %v) error in tree generation: %v", i, err)
						return
					}

					// restart subprotocol
					subProtocol, err = p.startSubProtocol(trees[i])
					if err != nil {
						err = fmt.Errorf("(subprotocol %v) error in restarting of subprotocol: %s", i, err)
						errChan <- err
						return
					}
					mut.Lock()
					cosiSubProtocols[i] = subProtocol
					mut.Unlock()
				case commitment := <-subProtocol.subCommitment:
					mut.Lock()
					subProtocolsCommitments[subProtocol] = commitment
					newMask, err := cosi.AggregateMasks(sharedMask.Mask(), commitment.Mask)
					if err != nil {
						mut.Unlock()
						err = fmt.Errorf("(subprotocol %v) error in aggregation of commitment masks: %s", i, err)
						errChan <- err
						return
					}
					err = sharedMask.SetMask(newMask)
					mut.Unlock()
					if err != nil {
						err = fmt.Errorf("(subprotocol %v) error in setting of shared masks: %s", i, err)
						errChan <- err
						return
					}
					if sharedMask.CountEnabled() >= p.Threshold-1 {
						thresholdReachedChan <- true
						return
					}
				case <-timeout:
					errChan <- fmt.Errorf("(subprotocol %v) didn't get commitment after timeout %v", i, p.Timeout)
					return
				}
			}
		}(i, subProtocol)
	}

	if p.Threshold == 0 {

	}

	thresholdReached := true
	if len(cosiSubProtocols) > 0 {
		thresholdReached = false
		select {
		case thresholdReached = <-thresholdReachedChan:
		case <-time.After(p.Timeout):
			log.Lvl2("Threshold", p.Threshold, "not reached at timeout, got", sharedMask.CountEnabled(), "commitments")
		}
	}

	close(closingChan)
	closingWg.Wait()
	close(thresholdReachedChan)
	close(errChan)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("failed to collect commitments with errors %v", errs)
	}

	if !thresholdReached {
		return nil, nil, fmt.Errorf("commitments not completed in time")
	}

	// extract protocols and commitments from map
	runningSubProtocols := make([]*SubFtCosi, 0, len(subProtocolsCommitments))
	commitments := make([]StructCommitment, 0, len(subProtocolsCommitments))
	for subProtocol, commitment := range subProtocolsCommitments {
		runningSubProtocols = append(runningSubProtocols, subProtocol)
		commitments = append(commitments, commitment)
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
	threshold := int(math.Ceil(float64(p.Threshold-1) / float64(p.NSubtrees)))
	if threshold > tree.Size()-1 {
		threshold = tree.Size() - 1
	}

	cosiSubProtocol.Threshold = threshold

	err = cosiSubProtocol.Start()
	if err != nil {
		return nil, err
	}

	return cosiSubProtocol, err
}
