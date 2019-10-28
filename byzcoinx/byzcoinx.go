// Package byzcoinx implements a PBFT-like protocol using collective signing.
//
// Please see https://github.com/dedis/cothority/blob/master/byzcoinx/README.md
// for details.
//
package byzcoinx

import (
	"fmt"
	"math"
	"time"

	"go.dedis.ch/cothority/v4/blscosi/protocol"
	"go.dedis.ch/cothority/v4/cosuite"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/ciphersuite"
	"go.dedis.ch/onet/v4/log"
	"golang.org/x/xerrors"
)

// ByzCoinX contains the state used to execute two rounds of blscosi.
type ByzCoinX struct {
	// the node we are represented-in
	*onet.TreeNodeInstance
	// Msg is the message that will be signed by cosigners
	Msg []byte
	// Data is used for verification only, not signed
	Data []byte
	// FinalSignature is output of the protocol, for the caller to read
	FinalSignatureChan chan FinalSignature
	// CreateProtocol stores a function pointer used to create the ftcosi
	// protocol
	CreateProtocol protocol.CreateProtocolFunction
	// Timeout is passed down to the blscosi protocol and used for waiting
	// for some of its messages.
	Timeout time.Duration
	// SubleaderFailures is the maximum number of attempts
	// when subleaders are failing
	SubleaderFailures int
	// Threshold is the number of nodes to reach for a signature to be valid
	Threshold int
	// prepCosiProtoName is the ftcosi protocol name for the prepare phase
	prepCosiProtoName string
	// commitCosiProtoName is the ftcosi protocol name for the commit phase
	commitCosiProtoName string
	// prepSigChan is the channel for reading the prepare phase signature
	prepSigChan chan ciphersuite.Signature

	suite cosuite.CoSiCipherSuite
	// nSubtrees is the number of subtrees used for the ftcosi protocols.
	nSubtrees int
}

// FinalSignature holds the message Msg and its signature
type FinalSignature struct {
	Msg []byte
	Sig *ciphersuite.RawSignature
}

type phase int

// VerifierFn is used to verify the final signature
type VerifierFn func(suite pairing.Suite, msg, sig []byte, pubkeys []kyber.Point) error

const (
	phasePrep phase = iota
	phaseCommit
)

// Start begins the BFTCoSi protocol by starting the prepare ftcosi.
func (bft *ByzCoinX) Start() error {
	if bft.CreateProtocol == nil {
		return fmt.Errorf("no CreateProtocol")
	}
	if bft.FinalSignatureChan == nil {
		return fmt.Errorf("no FinalSignatureChan")
	}

	// prepare phase (part 1)
	log.Lvl3("Starting prepare phase")
	prepProto, err := bft.initCosiProtocol(phasePrep)
	if err != nil {
		return err
	}

	err = prepProto.Start()
	if err != nil {
		return err
	}

	go func() {
		select {
		case tmpSig := <-prepProto.FinalSignature:
			bft.prepSigChan <- tmpSig
		case <-time.After(bft.Timeout / time.Duration(2) * time.Duration(bft.SubleaderFailures+1)):
			// Waiting for bft.Timeout is too long here but used as a safeguard in
			// case the prepProto does not return in time.
			log.Error(bft.ServerIdentity().Address, "timeout should not happen while waiting for signature")
			bft.prepSigChan <- nil
		}
	}()

	return nil
}

func (bft *ByzCoinX) initCosiProtocol(phase phase) (*protocol.BlsCosi, error) {
	var name string
	if phase == phasePrep {
		name = bft.prepCosiProtoName
	} else if phase == phaseCommit {
		name = bft.commitCosiProtoName
	} else {
		return nil, fmt.Errorf("invalid phase %v", phase)
	}

	pi, err := bft.CreateProtocol(name, bft.Tree())
	if err != nil {
		return nil, err
	}
	cosiProto := pi.(*protocol.BlsCosi)
	cosiProto.CreateProtocol = bft.CreateProtocol
	cosiProto.Msg = bft.Msg
	cosiProto.Data = bft.Data
	cosiProto.Threshold = bft.Threshold
	// For each of the prepare and commit phase we get half of the time.
	cosiProto.Timeout = bft.Timeout / 2

	if bft.SubleaderFailures > 0 {
		// Only update the parameter if it is defined, else keep the default
		// value.
		cosiProto.SubleaderFailures = bft.SubleaderFailures
	}

	cosiProto.SetNbrSubTree(bft.nSubtrees)

	return cosiProto, nil
}

// Dispatch is the main logic of the BFTCoSi protocol. It runs two CoSi
// protocols as the prepare and the commit phase of PBFT. Concretely, it does:
// 1, wait for the prepare phase to finish
// 2, check the signature
// 3, if it is, start the commit phase,
//    otherwise send an empty signature
// 4, wait for the commit phase to finish
// 5, send the final signature
func (bft *ByzCoinX) Dispatch() error {
	defer func() {
		close(bft.FinalSignatureChan)
		bft.Done()
	}()

	if !bft.IsRoot() {
		return fmt.Errorf("non-root should not start this protocol")
	}

	// prepare phase (part 2)
	prepSig := <-bft.prepSigChan

	aggKey, err := bft.suite.AggregatePublicKeys(bft.PublicKeys(), prepSig)
	if err != nil {
		log.Error("Couldn't aggregate the public key:", err)
		return nil
	}

	err = bft.suite.Verify(aggKey, prepSig, bft.Msg)
	if err != nil {
		log.Error("Signature verification failed on root during the prepare phase with error:", err)
		return nil
	}
	log.Lvl3("Finished prepare phase")

	// commit phase
	log.Lvl3("Starting commit phase")
	commitProto, err := bft.initCosiProtocol(phaseCommit)
	if err != nil {
		return err
	}

	err = commitProto.Start()
	if err != nil {
		return err
	}

	var commitSig ciphersuite.Signature
	select {
	case commitSig = <-commitProto.FinalSignature:
		log.Lvl3("Finished commit phase")
	case <-time.After(bft.Timeout / time.Duration(2) * time.Duration(bft.SubleaderFailures+1)):
		// Waiting for bft.Timeout is too long here but used as a safeguard in
		// case the commitProto does not return in time.
		log.Error(bft.ServerIdentity().Address, "timeout should not happen while waiting for signature")
	}

	aggKey, err = bft.suite.AggregatePublicKeys(bft.PublicKeys(), commitSig)
	if err != nil {
		return err
	}

	err = bft.suite.Verify(aggKey, commitSig, bft.Msg)
	if err != nil {
		return xerrors.New("commit signature is wrong")
	}

	if !bft.suite.VerifyThreshold(commitSig, bft.Threshold) {
		return xerrors.New("threshold not achieved")
	}

	bft.FinalSignatureChan <- FinalSignature{bft.Msg, commitSig.Raw()}
	return nil
}

// NewByzCoinX creates and initialises a ByzCoinX protocol.
func NewByzCoinX(n *onet.TreeNodeInstance, prepCosiProtoName, commitCosiProtoName string,
	suite cosuite.CoSiCipherSuite) (*ByzCoinX, error) {
	return &ByzCoinX{
		TreeNodeInstance: n,
		// we do not have Msg to make the protocol fail if it's not set
		FinalSignatureChan:  make(chan FinalSignature, 1),
		Data:                make([]byte, 0),
		prepCosiProtoName:   prepCosiProtoName,
		commitCosiProtoName: commitCosiProtoName,
		prepSigChan:         make(chan ciphersuite.Signature, 0),
		suite:               suite,
		// We set nSubtrees to the cube root of n to evenly distribute the load,
		// i.e. depth (=3) = log_f n, where f is the fan-out (branching factor).
		nSubtrees: int(math.Pow(float64(len(n.List())), 1.0/3.0)),
	}, nil
}

func makeProtocols(vf, ack protocol.VerificationFn, protoName string, suite cosuite.CoSiCipherSuite) map[string]onet.NewProtocol {

	protocolMap := make(map[string]onet.NewProtocol)

	prepCosiProtoName := protoName + "_cosi_prep"
	prepCosiSubProtoName := protoName + "_subcosi_prep"
	commitCosiProtoName := protoName + "_cosi_commit"
	commitCosiSubProtoName := protoName + "_subcosi_commit"

	protocolMap[protoName] = func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return NewByzCoinX(n, prepCosiProtoName, commitCosiProtoName, suite)
	}
	protocolMap[prepCosiProtoName] = func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewBlsCosi(n, vf, prepCosiSubProtoName, suite)
	}
	protocolMap[prepCosiSubProtoName] = func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewSubBlsCosi(n, vf, suite)
	}
	protocolMap[commitCosiProtoName] = func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewBlsCosi(n, ack, commitCosiSubProtoName, suite)
	}
	protocolMap[commitCosiSubProtoName] = func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewSubBlsCosi(n, ack, suite)
	}

	return protocolMap
}

// GlobalInitBFTCoSiProtocol creates and registers the protocols required to run
// BFTCoSi globally.
func GlobalInitBFTCoSiProtocol(suite cosuite.CoSiCipherSuite, vf, ack protocol.VerificationFn, protoName string) error {
	protocolMap := makeProtocols(vf, ack, protoName, suite)
	for protoName, proto := range protocolMap {
		if _, err := onet.GlobalProtocolRegister(protoName, proto); err != nil {
			return err
		}
	}
	return nil
}

// InitBFTCoSiProtocol creates and registers the protocols required to run
// BFTCoSi to the context c.
func InitBFTCoSiProtocol(suite cosuite.CoSiCipherSuite, c *onet.Context, vf, ack protocol.VerificationFn, protoName string) error {
	protocolMap := makeProtocols(vf, ack, protoName, suite)
	for protoName, proto := range protocolMap {
		if _, err := c.ProtocolRegister(protoName, proto); err != nil {
			return err
		}
	}
	return nil
}

// FaultThreshold computes the number of faults that byzcoinx tolerates.
func FaultThreshold(n int) int {
	return protocol.DefaultFaultyThreshold(n)
}

// Threshold computes the number of nodes needed for successful operation.
func Threshold(n int) int {
	return protocol.DefaultThreshold(n)
}
