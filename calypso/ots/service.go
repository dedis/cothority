package ots

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"golang.org/x/xerrors"
	"sync"
)

// Used for tests
var otsID onet.ServiceID

const ServiceName = "OTS"

// TODO: Fix contract names
func init() {
	var err error
	otsID, err = onet.RegisterNewService(ServiceName, newService)
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
}

type Service struct {
	*onet.ServiceProcessor
	storage           *storage
	genesisBlocks     map[string]*skipchain.SkipBlock
	genesisBlocksLock sync.Mutex
}

// vData is sent to all nodes when re-encryption takes place.
type vData struct {
	Read  *byzcoin.Proof
	Write *byzcoin.Proof
}

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		genesisBlocks:    make(map[string]*skipchain.SkipBlock),
	}
	//if err := s.RegisterHandlers(s.VerifyWrite, s.DecryptKey); err != nil {
	//	return nil, xerrors.New("Couldn't register messages")
	//}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, xerrors.Errorf("loading configuration: %v", err)
	}
	return s, nil
}
