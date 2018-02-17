package omnicon

import (
	"fmt"
	"time"

	"github.com/dedis/cothority/cosi/protocol"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

// ProtocolBFTCoSi contains the state used in the execution of the BFTCoSi
// protocol. It is also known as OmniCon, which is described in the OmniLedger
// paper - https://eprint.iacr.org/2017/406
type ProtocolBFTCoSi struct {
	// the node we are represented-in
	*onet.TreeNodeInstance
	// Proposal is the message that will be signed by cosigners
	Proposal []byte
	// FinalSignature is output of the protocol, for the caller to read
	FinalSignature chan []byte
	// CreateProtocol TODO
	CreateProtocol protocol.CreateProtocolFunction
	// cosiProtocolName is the name given to the main cosi protocol
	cosiProtocolName string
	// protocolTimeout define the timeout duration
	protocolTimeout time.Duration
	// nbrFault TODO
	nbrFault int
	// prepSigChan TODO
	prepSigChan chan []byte
}

// Start begins the BFTCoSi protocol by starting the prepare cosi.
func (bft *ProtocolBFTCoSi) Start() error {
	// prepare phase (part 1)
	log.Lvl3("Starting prepare phase")
	prepProto, err := bft.initCosiProtocol()
	if err != nil {
		return err
	}

	err = prepProto.Start()
	if err != nil {
		return err
	}

	go func() {
		bft.prepSigChan <- <-prepProto.FinalSignature
	}()

	return nil
}

func (bft *ProtocolBFTCoSi) initCosiProtocol() (*protocol.CoSiRootNode, error) {
	pi, err := bft.CreateProtocol(bft.cosiProtocolName, bft.Tree()) // TODO bft.Tree() is ok?
	if err != nil {
		return nil, err
	}
	cosiProto := pi.(*protocol.CoSiRootNode)
	cosiProto.CreateProtocol = bft.CreateProtocol
	cosiProto.Proposal = bft.Proposal
	cosiProto.NSubtrees = 3 // TODO how to compute this?
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
func (bft *ProtocolBFTCoSi) Dispatch() error {

	if !bft.IsRoot() {
		return fmt.Errorf("non-root should not start this protocol")
	}

	// prepare phase (part 2)
	prepSig := <-bft.prepSigChan
	suite := bft.Suite().(suites.Suite)
	err := cosi.Verify(suite, bft.Roster().Publics(), bft.Proposal, prepSig, cosi.NewThresholdPolicy(bft.nbrFault))
	if err != nil {
		bft.FinalSignature <- nil
		return nil
	}
	log.Lvl3("Finished prepare phase")

	// commit phase
	log.Lvl3("Starting commit phase")
	commitProto, err := bft.initCosiProtocol()
	if err != nil {
		return err
	}

	err = commitProto.Start()
	if err != nil {
		return err
	}

	commitSig := <-commitProto.FinalSignature
	log.Lvl3("Finished commit phase")

	bft.FinalSignature <- commitSig
	return nil
}

// NewBFTCoSiProtocol TODO
func NewBFTCoSiProtocol(n *onet.TreeNodeInstance, vf protocol.VerificationFn, cosiProtocolName string) (*ProtocolBFTCoSi, error) {
	return &ProtocolBFTCoSi{
		TreeNodeInstance: n,
		Proposal:         make([]byte, 0),
		FinalSignature:   make(chan []byte, 0),
		cosiProtocolName: cosiProtocolName,
		protocolTimeout:  time.Second * 10, // TODO make it configurable
		nbrFault:         1,                // TODO compute this
		prepSigChan:      make(chan []byte, 0),
	}, nil
}

func makeProtocols(vf protocol.VerificationFn, protoName string) (
	string, string, string,
	onet.NewProtocol, onet.NewProtocol, onet.NewProtocol) {

	cosiProtoName := protoName + "_cosi"
	cosiSubProtoName := protoName + "_subcosi"

	// the protocol names must be in sync here...
	bftProto := func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n, vf, cosiProtoName)
	}
	cosiProto := func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewProtocol(n, vf, cosiSubProtoName)
	}
	cosiSubProto := func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewSubProtocol(n, vf)
	}

	return protoName, cosiProtoName, cosiSubProtoName, bftProto, cosiProto, cosiSubProto
}

// GlobalInitBFTCoSiProtocol creates and registers the protocols required to run
// BFTCoSi globally.
func GlobalInitBFTCoSiProtocol(vf protocol.VerificationFn, protoName string) error {
	cosiProtoName, cosiSubProtoName, bftProtoName, cosiProto, cosiSubProto, bftProto := makeProtocols(vf, protoName)

	var err error
	_, err = onet.GlobalProtocolRegister(cosiProtoName, cosiProto)
	if err != nil {
		return err
	}
	_, err = onet.GlobalProtocolRegister(cosiSubProtoName, cosiSubProto)
	if err != nil {
		return err
	}
	_, err = onet.GlobalProtocolRegister(bftProtoName, bftProto)
	if err != nil {
		return err
	}
	return nil
}

// InitBFTCoSiProtocol creates and registers the protocols required to run
// BFTCoSi to the context c.
func InitBFTCoSiProtocol(c onet.Context, vf protocol.VerificationFn, protoName string) error {
	cosiProtoName, cosiSubProtoName, bftProtoName, cosiProto, cosiSubProto, bftProto := makeProtocols(vf, protoName)

	// register the protocols
	var err error
	_, err = c.ProtocolRegister(cosiProtoName, cosiProto)
	if err != nil {
		return err
	}
	_, err = c.ProtocolRegister(cosiSubProtoName, cosiSubProto)
	if err != nil {
		return err
	}
	_, err = c.ProtocolRegister(bftProtoName, bftProto)
	if err != nil {
		return err
	}
	return nil
}
