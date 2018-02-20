package protocol

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/kyber"
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
	network.RegisterMessages(Announcement{}, Commitment{}, Challenge{}, Response{}, Stop{})
}

// CoSiRootNode holds the parameters of the protocol.
// It also defines a channel that will receive the final signature.
// This protocol should only exist on the root node.
type CoSiRootNode struct {
	*onet.TreeNodeInstance

	NSubtrees      int
	Msg            []byte
	Data           []byte
	CreateProtocol CreateProtocolFunction
	Timeout        time.Duration
	FinalSignature chan []byte

	publics         []kyber.Point
	hasStopped      bool //used since Shutdown can be called multiple time
	startChan       chan bool
	subProtocolName string
	verificationFn  VerificationFn
}

// CreateProtocolFunction is a function type which creates a new protocol
// used in CoSiRootNode protocol for creating sub leader protocols.
type CreateProtocolFunction func(name string, t *onet.Tree) (onet.ProtocolInstance, error)

// NewDefaultProtocol is the default protocol function used for registration
// with an always-true verification.
func NewDefaultProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewProtocol(n, vf, DefaultSubProtocolName)
}

// GlobalRegisterDefaultProtocols is used to register the protocols before use,
// most likely in an init function.
func GlobalRegisterDefaultProtocols() {
	onet.GlobalProtocolRegister(DefaultProtocolName, NewDefaultProtocol)
	onet.GlobalProtocolRegister(DefaultSubProtocolName, NewDefaultSubProtocol)
}

// NewProtocol method is used to define the protocol.
func NewProtocol(n *onet.TreeNodeInstance, vf VerificationFn, subProtocolName string) (onet.ProtocolInstance, error) {

	var list []kyber.Point
	for _, t := range n.Tree().List() {
		list = append(list, t.ServerIdentity.Public)
	}

	c := &CoSiRootNode{
		TreeNodeInstance: n,
		FinalSignature:   make(chan []byte),
		Data:             make([]byte, 0),
		publics:          list,
		hasStopped:       false,
		startChan:        make(chan bool),
		verificationFn:   vf,
		subProtocolName:  subProtocolName,
	}

	return c, nil
}

// Shutdown stops the protocol
func (p *CoSiRootNode) Shutdown() error {
	if !p.hasStopped {
		p.hasStopped = true
	}
	return nil
}

// Dispatch is the main method of the protocol, defining the root node behaviour
// and sequential handling of subprotocols.
func (p *CoSiRootNode) Dispatch() error {
	if !p.IsRoot() {
		return nil
	}

	verifyChan := make(chan bool, 1)
	go func() {
		log.Lvl3(p.ServerIdentity().Address, "starting verification")
		verifyChan <- p.verificationFn(p.Msg, p.Data)
	}()

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

	log.Lvl1("leader protocol started")

	// generate trees
	nNodes := p.Tree().Size()
	trees, err := GenTrees(p.Tree().Roster, nNodes, p.NSubtrees)
	if err != nil {
		return fmt.Errorf("error in tree generation: %s", err)
	}

	// if one node, sign without subprotocols
	if nNodes == 1 {
		trees = make([]*onet.Tree, 0)
	}

	// start all subprotocols
	cosiSubProtocols := make([]*CoSiSubProtocolNode, len(trees))
	for i, tree := range trees {
		cosiSubProtocols[i], err = p.startSubProtocol(tree)
		if err != nil {
			return err
		}
	}
	log.Lvl3("all protocols started")

	commitments, runningSubProtocols, err := p.collectCommitments(trees, cosiSubProtocols)
	if err != nil {
		return err
	}

	suite, ok := p.Suite().(cosi.Suite)
	if !ok {
		return errors.New("not a cosi suite")
	}

	// generate challenge
	log.Lvl3("root-node generating global challenge")
	secret, commitment, finalMask, err := generateCommitmentAndAggregate(suite, p.TreeNodeInstance, p.publics, commitments)
	if err != nil {
		return err
	}

	cosiChallenge, err := cosi.Challenge(suite, commitment, finalMask.AggregatePublic, p.Msg)
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
	var responsesMut sync.Mutex
	var responsesWg sync.WaitGroup
	for _, cosiSubProtocol := range runningSubProtocols {
		responsesWg.Add(1)
		go func(subProto *CoSiSubProtocolNode) {
			defer responsesWg.Done()
			select {
			case response := <-subProto.subResponse:
				responsesMut.Lock()
				responses = append(responses, response)
				responsesMut.Unlock()
			case <-time.After(p.Timeout):
				log.Error("didn't finish in time")
			}
		}(cosiSubProtocol)
	}
	responsesWg.Wait()
	if len(responses) != len(runningSubProtocols) {
		return fmt.Errorf("did not get all the responses")
	}

	// signs the proposal
	if !<-verifyChan {
		// root should not fail the verification otherwise it would not have
		// started the protocol
		p.FinalSignature <- nil
		return fmt.Errorf("verification failed on root node")
	}
	response, err := generateResponse(suite, p.TreeNodeInstance, responses, secret, cosiChallenge)
	if err != nil {
		return err
	}
	log.Lvl3(p.ServerIdentity().Address, "starts final signature")
	var signature []byte
	signature, err = cosi.Sign(suite, commitment, response, finalMask)
	if err != nil {
		log.Print(err)
		return err
	}
	p.FinalSignature <- signature

	log.Lvl3("Root-node is done without errors")
	return nil
}

func (p *CoSiRootNode) collectCommitments(trees []*onet.Tree, cosiSubProtocols []*CoSiSubProtocolNode) ([]StructCommitment, []*CoSiSubProtocolNode, error) {
	// get all commitments, restart subprotocols where subleaders do not respond
	var mut sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, 1) // only keep one error
	commitments := make([]StructCommitment, 0)
	runningSubProtocols := make([]*CoSiSubProtocolNode, 0)

	for i, subProtocol := range cosiSubProtocols {
		wg.Add(1)
		go func(i int, subProtocol *CoSiSubProtocolNode) {
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
					trees[i], err = GenSubtree(trees[i].Roster, newSubleaderID)
					if err != nil {
						select {
						case errChan <- err:
						default:
						}
						return
					}

					// restart subprotocol
					subProtocol, err = p.startSubProtocol(trees[i])
					if err != nil {
						err = fmt.Errorf("error in restarting of subprotocol: %s", err)
						select {
						case errChan <- err:
						default:
						}
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
					err := fmt.Errorf("didn't get commitment after timeout %v", p.Timeout)
					select {
					case errChan <- err:
					default:
					}
					return
				}
			}
		}(i, subProtocol)
	}
	wg.Wait()

	select {
	case err := <-errChan:
		return nil, nil, err
	default:
		return commitments, runningSubProtocols, nil
	}
}

// Start is done only by root and starts the protocol.
// It also verifies that the protocol has been correctly parameterized.
func (p *CoSiRootNode) Start() error {
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
func (p *CoSiRootNode) startSubProtocol(tree *onet.Tree) (*CoSiSubProtocolNode, error) {

	pi, err := p.CreateProtocol(p.subProtocolName, tree)
	if err != nil {
		return nil, err
	}

	cosiSubProtocol := pi.(*CoSiSubProtocolNode)
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
