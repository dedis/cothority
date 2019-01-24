// Package byzcoinx implements a PBFT-like protocol using collective signing.
//
// Please see https://github.com/dedis/cothority/blob/master/byzcoinx/README.md
// for details.
//
package byzcoinx

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dedis/cothority/blscosi/protocol"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

// ByzCoinX contains the state used in the execution of the BFTCoSi
// protocol. It is also known as OmniCon, which is described in the OmniLedger
// paper - https://eprint.iacr.org/2017/406
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
	// Timeout is passed down to the ftcosi protocol and used for waiting
	// for some of its messages.
	Timeout time.Duration
	// Threshold is the number of nodes to reach for a signature to be valid
	Threshold int
	// prepCosiProtoName is the ftcosi protocol name for the prepare phase
	prepCosiProtoName string
	// commitCosiProtoName is the ftcosi protocol name for the commit phase
	commitCosiProtoName string
	// prepSigChan is the channel for reading the prepare phase signature
	prepSigChan chan protocol.BlsSignature
	// publics is the list of public keys
	publics []kyber.Point
	// suite is the ftcosi.Suite, which may be different from the suite used
	// in the protocol because we need sha512 for the hash function so that
	// the signature can be verified using eddsa.Verify.
	suite *pairing.SuiteBn256
	// nSubtrees is the number of subtrees used for the ftcosi protocols.
	nSubtrees int
}

// FinalSignature holds the message Msg and its signature
type FinalSignature struct {
	Msg []byte
	Sig protocol.BlsSignature
}

type phase int

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
		case <-time.After(bft.Timeout):
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
	defer bft.Done()

	if !bft.IsRoot() {
		return fmt.Errorf("non-root should not start this protocol")
	}

	// prepare phase (part 2)
	prepSig := <-bft.prepSigChan
	err := prepSig.Verify(bft.suite, bft.Msg, bft.publics)
	if err != nil {
		log.Lvl2("Signature verification failed on root during the prepare phase with error:", err)
		bft.FinalSignatureChan <- FinalSignature{nil, nil}
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

	var commitSig protocol.BlsSignature
	select {
	case commitSig = <-commitProto.FinalSignature:
		log.Lvl3("Finished commit phase")
	case <-time.After(bft.Timeout):
		// Waiting for bft.Timeout is too long here but used as a safeguard in
		// case the commitProto does not return in time.
		log.Error(bft.ServerIdentity().Address, "timeout should not happen while waiting for signature")
	}

	err = commitSig.Verify(bft.suite, bft.Msg, bft.publics)
	if err != nil {
		bft.FinalSignatureChan <- FinalSignature{nil, nil}
		return errors.New("Commit signature is wrong")
	}

	bft.FinalSignatureChan <- FinalSignature{bft.Msg, commitSig}
	return nil
}

// NewByzCoinX creates and initialises a ByzCoinX protocol.
func NewByzCoinX(n *onet.TreeNodeInstance, prepCosiProtoName, commitCosiProtoName string,
	suite *pairing.SuiteBn256) (*ByzCoinX, error) {
	return &ByzCoinX{
		TreeNodeInstance: n,
		// we do not have Msg to make the protocol fail if it's not set
		FinalSignatureChan:  make(chan FinalSignature, 1),
		Data:                make([]byte, 0),
		prepCosiProtoName:   prepCosiProtoName,
		commitCosiProtoName: commitCosiProtoName,
		prepSigChan:         make(chan protocol.BlsSignature, 0),
		publics:             n.Publics(),
		suite:               suite,
		// We set nSubtrees to the cube root of n to evenly distribute the load,
		// i.e. depth (=3) = log_f n, where f is the fan-out (branching factor).
		nSubtrees: int(math.Pow(float64(len(n.List())), 1.0/3.0)),
	}, nil
}

func makeProtocols(vf, ack protocol.VerificationFn, protoName string, suite *pairing.SuiteBn256) map[string]onet.NewProtocol {

	protocolMap := make(map[string]onet.NewProtocol)

	prepCosiProtoName := protoName + "_cosi_prep"
	prepCosiSubProtoName := protoName + "_subcosi_prep"
	commitCosiProtoName := protoName + "_cosi_commit"
	commitCosiSubProtoName := protoName + "_subcosi_commit"

	bftProto := func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return NewByzCoinX(n, prepCosiProtoName, commitCosiProtoName, suite)
	}
	protocolMap[protoName] = bftProto

	prepCosiProto := func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewBlsCosi(n, vf, prepCosiSubProtoName, suite)
	}
	protocolMap[prepCosiProtoName] = prepCosiProto

	prepCosiSubProto := func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewSubBlsCosi(n, vf, suite)
	}
	protocolMap[prepCosiSubProtoName] = prepCosiSubProto

	commitCosiProto := func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewBlsCosi(n, ack, commitCosiSubProtoName, suite)
	}
	protocolMap[commitCosiProtoName] = commitCosiProto

	commitCosiSubProto := func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewSubBlsCosi(n, ack, suite)
	}
	protocolMap[commitCosiSubProtoName] = commitCosiSubProto

	return protocolMap
}

// GlobalInitBFTCoSiProtocol creates and registers the protocols required to run
// BFTCoSi globally.
func GlobalInitBFTCoSiProtocol(suite *pairing.SuiteBn256, vf, ack protocol.VerificationFn, protoName string) error {
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
func InitBFTCoSiProtocol(suite *pairing.SuiteBn256, c *onet.Context, vf, ack protocol.VerificationFn, protoName string) error {
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
	return (n - 1) / 3
}

// Threshold computes the number of nodes needed for successful operation.
func Threshold(n int) int {
	return n - FaultThreshold(n)
}
