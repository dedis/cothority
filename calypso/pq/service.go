package pq

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"golang.org/x/xerrors"
)

// Used for tests
var ocspqID onet.ServiceID

// ServiceName of the XXX.
const ServiceName = "OCSPQ"

func init() {
	var err error
	ocspqID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &vData{})

	//err = byzcoin.RegisterGlobalContract(ContractWriteID, contractWriteFromBytes)
	//if err != nil {
	//	log.ErrFatal(err)
	//}
	//err = byzcoin.RegisterGlobalContract(ContractReadID, contractReadFromBytes)
	//if err != nil {
	//	log.ErrFatal(err)
	//}
	//err = byzcoin.RegisterGlobalContract(ContractLongTermSecretID, contractLTSFromBytes)
	//if err != nil {
	//	log.ErrFatal(err)
	//}
}

type Service struct {
	*onet.ServiceProcessor
	storage *storage
}

// vData is sent to all nodes when re-encryption takes place. If Ephemeral
// is non-nil, Signature needs to hold a valid signature from the reader
// in the Proof.
type vData struct {
	Proof     byzcoin.Proof
	Ephemeral kyber.Point
	Signature *darc.Signature
}

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	//if err := s.RegisterHandlers(s.CreateLTS, s.ReshareLTS, s.DecryptKey,
	//	s.GetLTSReply, s.Authorise, s.Authorize, s.updateValidPeers); err != nil {
	//	return nil, xerrors.New("couldn't register messages")
	//}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, xerrors.Errorf("loading configuration: %v", err)
	}

	// Initialize the sets of valid peers for all existing LTS
	//for ltsID, roster := range s.storage.Rosters {
	//	s.SetValidPeers(s.NewPeerSetID(ltsID[:]), roster.List)
	//}

	return s, nil
}
