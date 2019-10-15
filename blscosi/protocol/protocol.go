// Package protocol implements the BLS protocol using a main protocol and multiple
// subprotocols, one for each substree.
//
// Deprecated: use the bdnproto instead to be robust against the rogue public-key
// attack described here: https://crypto.stanford.edu/~dabo/pubs/papers/BLSmultisig.html
package protocol

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing"
	"go.dedis.ch/kyber/v4/sign"
	"go.dedis.ch/kyber/v4/sign/bls"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"
)

const defaultTimeout = 10 * time.Second
const defaultSubleaderFailures = 2

// VerificationFn is called on every node. Where msg is the message that is
// co-signed and the data is additional data for verification.
type VerificationFn func(msg, data []byte) bool

// VerifyFn is called to verify a single signature
type VerifyFn func(suite pairing.Suite, pub kyber.Point, msg []byte, sig []byte) error

// SignFn is called to sign the message
type SignFn func(suite pairing.Suite, secret kyber.Scalar, msg []byte) ([]byte, error)

// AggregateFn is called to aggregate multiple signatures and to produce a
// mask of the peer's participation
type AggregateFn func(suite pairing.Suite, mask *sign.Mask, sigs [][]byte) ([]byte, error)

// BlsCosi holds the parameters of the protocol.
// It also defines a channel that will receive the final signature.
// This protocol should only exist on the root node.
type BlsCosi struct {
	*onet.TreeNodeInstance
	Msg            []byte
	Data           []byte
	CreateProtocol CreateProtocolFunction
	Verify         VerifyFn
	Sign           SignFn
	Aggregate      AggregateFn
	// Timeout is not a global timeout for the protocol, but a timeout used
	// for waiting for responses for sub protocols.
	Timeout           time.Duration
	SubleaderFailures int
	Threshold         int
	FinalSignature    chan []byte // final signature that is sent back to client

	stoppedOnce      sync.Once
	subProtocolsLock sync.Mutex
	subProtocols     []*SubBlsCosi
	subProtocolName  string
	verificationFn   VerificationFn
	suite            *pairing.SuiteBn256
	subTrees         BlsProtocolTree
}

// CreateProtocolFunction is a function type which creates a new protocol
// used in BlsCosi protocol for creating sub leader protocols.
type CreateProtocolFunction func(name string, t *onet.Tree) (onet.ProtocolInstance, error)

// NewDefaultProtocol is the default protocol function used for registration
// with an always-true verification.
// Called by GlobalRegisterDefaultProtocols
func NewDefaultProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewBlsCosi(n, vf, DefaultSubProtocolName, pairing.NewSuiteBn256())
}

// GlobalRegisterDefaultProtocols is used to register the protocols before use,
// most likely in an init function.
func GlobalRegisterDefaultProtocols() {
	onet.GlobalProtocolRegister(DefaultProtocolName, NewDefaultProtocol)
	onet.GlobalProtocolRegister(DefaultSubProtocolName, NewDefaultSubProtocol)
}

// DefaultFaultyThreshold computes the maximum number of faulty nodes
func DefaultFaultyThreshold(n int) int {
	return (n - 1) / 3
}

// DefaultThreshold computes the minimal threshold authorized using
// the formula 3f+1
func DefaultThreshold(n int) int {
	return n - DefaultFaultyThreshold(n)
}

// NewBlsCosi method is used to define the blscosi protocol.
func NewBlsCosi(n *onet.TreeNodeInstance, vf VerificationFn, subProtocolName string, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	nNodes := len(n.Roster().List)
	c := &BlsCosi{
		TreeNodeInstance:  n,
		FinalSignature:    make(chan []byte, 1),
		Timeout:           defaultTimeout,
		SubleaderFailures: defaultSubleaderFailures,
		Threshold:         DefaultThreshold(nNodes),
		Sign:              bls.Sign,
		Verify:            bls.Verify,
		Aggregate:         aggregate,
		verificationFn:    vf,
		subProtocolName:   subProtocolName,
		suite:             suite,
	}

	// the default number of subtree is the square root to
	// distribute the nodes evenly
	c.SetNbrSubTree(int(math.Sqrt(float64(nNodes - 1))))

	return c, nil
}

// SetNbrSubTree generates N new subtrees that will be used
// for the protocol
func (p *BlsCosi) SetNbrSubTree(nbr int) error {
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
		return fmt.Errorf("error in tree generation: %s", err.Error())
	}

	return nil
}

// Shutdown stops the protocol
func (p *BlsCosi) Shutdown() error {
	p.stoppedOnce.Do(func() {
		p.subProtocolsLock.Lock()
		for _, subCosi := range p.subProtocols {
			// we're stopping the root thus it will stop the children
			// by itself using a broadcasted message
			subCosi.Shutdown()
		}
		p.subProtocolsLock.Unlock()
		close(p.FinalSignature)
	})

	log.Lvl3("BLS CoSi ends")
	return nil
}

// Dispatch is not used for the main protocol
func (p *BlsCosi) Dispatch() error {
	// This protocol relies only on the start call to spin up sub-protocols
	return nil
}

// Start is done only by root and starts the protocol.
// It also verifies that the protocol has been correctly parameterized.
func (p *BlsCosi) Start() error {
	if !p.IsRoot() {
		p.Done()
		return errors.New("node must be the root")
	}

	err := p.checkIntegrity()
	if err != nil {
		p.Done()
		return err
	}

	log.Lvlf3("Starting BLS CoSi on %v", p.ServerIdentity())

	go p.runSubProtocols()

	return nil
}

func (p *BlsCosi) runSubProtocols() {
	defer p.Done()

	// Verification of the data is done before contacting the children
	if ok := p.verificationFn(p.Msg, p.Data); !ok {
		// root should not fail the verification otherwise it would not have started the protocol
		log.Errorf("verification failed on root node")
		return
	}

	// start all subprotocols
	p.subProtocolsLock.Lock()
	p.subProtocols = make([]*SubBlsCosi, len(p.subTrees))
	for i, tree := range p.subTrees {
		log.Lvlf3("Invoking start sub protocol on %v", tree.Root.ServerIdentity)
		var err error
		p.subProtocols[i], err = p.startSubProtocol(tree)
		if err != nil {
			p.subProtocolsLock.Unlock()
			log.Error(err)
			return
		}
	}
	p.subProtocolsLock.Unlock()
	log.Lvl3(p.ServerIdentity().Address, "all protocols started")

	// Wait and collect all the signature responses
	responses, err := p.collectSignatures()
	if err != nil {
		log.Error(err)
		return
	}

	log.Lvl3(p.ServerIdentity().Address, "collected all signature responses")

	// generate root signature
	sig, err := p.generateSignature(responses)
	if err != nil {
		log.Error(err)
		return
	}

	p.FinalSignature <- sig
}

// checkIntegrity checks if the protocol has been instantiated with
// correct parameters
func (p *BlsCosi) checkIntegrity() error {
	if p.Msg == nil {
		return fmt.Errorf("no proposal msg specified")
	}
	if p.CreateProtocol == nil {
		return fmt.Errorf("no create protocol function specified")
	}
	if p.verificationFn == nil {
		return fmt.Errorf("verification function cannot be nil")
	}
	if p.subProtocolName == "" {
		return fmt.Errorf("sub-protocol name cannot be empty")
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

// startSubProtocol creates, parametrize and starts a subprotocol on a given tree
// and returns the started protocol.
func (p *BlsCosi) startSubProtocol(tree *onet.Tree) (*SubBlsCosi, error) {
	pi, err := p.CreateProtocol(p.subProtocolName, tree)
	if err != nil {
		return nil, err
	}
	cosiSubProtocol := pi.(*SubBlsCosi)
	cosiSubProtocol.Msg = p.Msg
	cosiSubProtocol.Data = p.Data
	// Fail fast enough if the subleader is failing to try
	// at least three leaves as new subleader
	cosiSubProtocol.Timeout = p.Timeout / time.Duration(p.SubleaderFailures+1)
	// Give one leaf for free but as we don't know how many leaves
	// could fail from the other trees, we need as much as possible
	// responses. The main protocol will deal with early answers.
	cosiSubProtocol.Threshold = tree.Size() - 1

	log.Lvlf3("Starting sub protocol with subleader %v", tree.Root.Children[0].ServerIdentity)
	err = cosiSubProtocol.Start()
	if err != nil {
		return nil, err
	}

	return cosiSubProtocol, err
}

// Collect signatures from each sub-leader, restart whereever sub-leaders fail to respond.
// The collected signatures are already aggregated for a particular group
func (p *BlsCosi) collectSignatures() (ResponseMap, error) {
	p.subProtocolsLock.Lock()
	numSubProtocols := len(p.subProtocols)
	responsesChan := make(chan StructResponse, numSubProtocols)
	errChan := make(chan error, numSubProtocols)
	closeChan := make(chan bool)
	// force to stop pending selects in case of timeout or quick answers
	defer func() { close(closeChan) }()

	for i, subProtocol := range p.subProtocols {
		go func(i int, subProtocol *SubBlsCosi) {
			for {
				// this select doesn't have any timeout because a global is used
				// when aggregating the response. The close channel will act as
				// a timeout if one subprotocol hangs.
				select {
				case <-closeChan:
					// quick answer/failure
					return
				case <-subProtocol.subleaderNotResponding:
					subleaderID := p.subTrees[i].Root.Children[0].RosterIndex
					log.Lvlf2("(subprotocol %v) subleader with id %d failed, restarting subprotocol", i, subleaderID)

					// generate new tree by adding the current subleader to the end of the
					// leafs and taking the first leaf for the new subleader.
					nodes := []int{p.subTrees[i].Root.RosterIndex}
					for _, child := range p.subTrees[i].Root.Children[0].Children {
						nodes = append(nodes, child.RosterIndex)
					}

					if len(nodes) < 2 || subleaderID > nodes[1] {
						errChan <- fmt.Errorf("(subprotocol %v) failed with every subleader, ignoring this subtree",
							i)
						return
					}
					nodes = append(nodes, subleaderID)

					var err error
					p.subTrees[i], err = genSubtree(p.subTrees[i].Roster, nodes)
					if err != nil {
						errChan <- fmt.Errorf("(subprotocol %v) error in tree generation: %v", i, err)
						return
					}

					// restart subprotocol
					// send stop signal to old protocol
					subProtocol.HandleStop(StructStop{subProtocol.TreeNode(), Stop{}})
					subProtocol, err = p.startSubProtocol(p.subTrees[i])
					if err != nil {
						errChan <- fmt.Errorf("(subprotocol %v) error in restarting of subprotocol: %s", i, err)
						return
					}

					p.subProtocolsLock.Lock()
					p.subProtocols[i] = subProtocol
					p.subProtocolsLock.Unlock()
				case response := <-subProtocol.subResponse:
					responsesChan <- response
					return
				}
			}
		}(i, subProtocol)
	}
	p.subProtocolsLock.Unlock()

	// handle answers from all parallel threads
	responseMap := make(ResponseMap)
	numSignature := 0
	numFailure := 0
	timeout := time.After(p.Timeout)
	for numSubProtocols > 0 && numSignature < p.Threshold-1 && !p.checkFailureThreshold(numFailure) {
		select {
		case res := <-responsesChan:
			publics := p.Publics()
			mask, err := sign.NewMask(p.suite, publics, nil)
			if err != nil {
				return nil, err
			}
			err = mask.SetMask(res.Mask)
			if err != nil {
				return nil, err
			}

			public, index := searchPublicKey(p.TreeNodeInstance, res.ServerIdentity)
			if public != nil {
				if _, ok := responseMap[index]; !ok {
					count := mask.CountEnabled()
					numSignature += count
					numFailure += res.SubtreeCount() + 1 - count

					responseMap[index] = &res.Response
				}
			}
		case err := <-errChan:
			err = fmt.Errorf("error in getting responses: %s", err)
			return nil, err
		case <-timeout:
			// here we use the entire timeout so that the protocol won't take
			// more than Timeout + root computation time
			return nil, fmt.Errorf("not enough replies from nodes at timeout %v "+
				"for Threshold %d, got %d responses for %d requests", p.Timeout,
				p.Threshold, numSignature, len(p.Roster().List)-1)
		}
	}

	if p.checkFailureThreshold(numFailure) {
		return nil, fmt.Errorf("too many refusals (got %d), the threshold of %d cannot be achieved",
			numFailure, p.Threshold)
	}

	return responseMap, nil
}

// Sign the message with this node and aggregates with all child signatures (in structResponses)
// Also aggregates the child bitmasks
func (p *BlsCosi) generateSignature(responses ResponseMap) (BlsSignature, error) {
	publics := p.Publics()

	//generate personal mask
	personalMask, err := sign.NewMask(p.suite, publics, p.Public())
	if err != nil {
		return nil, err
	}

	// generate personal signature and append to other sigs
	personalSig, err := p.Sign(p.suite, p.Private(), p.Msg)
	if err != nil {
		return nil, err
	}

	// even if there is only one, it is aggregated to include potential processing
	// done during the aggregation
	agg, err := p.Aggregate(p.suite, personalMask, [][]byte{personalSig})
	if err != nil {
		return nil, err
	}

	_, index := searchPublicKey(p.TreeNodeInstance, p.ServerIdentity())
	// fill the map with the Root signature
	responses[index] = &Response{
		Mask:      personalMask.Mask(),
		Signature: agg,
	}

	// Aggregate all signatures
	sig, err := p.makeAggregateResponse(p.suite, publics, responses)
	if err != nil {
		log.Lvlf3("%v failed to create aggregate signature", p.ServerIdentity())
		return nil, err
	}

	return sig, err
}

// searchPublicKey looks for the corresponding server identity in the roster
// to prevent forged identity to be used
func searchPublicKey(p *onet.TreeNodeInstance, servID *network.ServerIdentity) (kyber.Point, int) {
	for idx, si := range p.Roster().List {
		if si.Equal(servID) {
			return p.NodePublic(si), idx
		}
	}

	return nil, -1
}

// makeAggregateResponse takes all the responses from the children and the subleader to
// aggregate the signature and the mask
func (p *BlsCosi) makeAggregateResponse(suite pairing.Suite, publics []kyber.Point, responses ResponseMap) (BlsSignature, error) {
	finalMask, err := sign.NewMask(suite, publics, nil)
	if err != nil {
		return nil, err
	}
	finalSignature := suite.G1().Point()

	for _, res := range responses {
		if res == nil || len(res.Signature) == 0 {
			continue
		}

		sig, err := res.Signature.Point(suite)
		if err != nil {
			return nil, err
		}
		finalSignature = finalSignature.Add(finalSignature, sig)

		err = finalMask.Merge(res.Mask)
		if err != nil {
			return nil, err
		}
	}

	sig, err := finalSignature.MarshalBinary()
	if err != nil {
		return nil, err
	}

	log.Lvlf3("%v is done aggregating signatures with total of %d signatures", p.ServerIdentity(), finalMask.CountEnabled())

	return append(sig, finalMask.Mask()...), nil
}

func aggregate(suite pairing.Suite, mask *sign.Mask, sigs [][]byte) ([]byte, error) {
	return bls.AggregateSignatures(suite, sigs...)
}
