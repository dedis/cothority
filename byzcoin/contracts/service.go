package contracts

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

// This service is only used because we need to register our contracts to
// the ByzCoin service. So we create this stub and add contracts to it
// from the `contracts` directory.

func init() {
	_, err := onet.RegisterNewService("contracts", newService)
	log.ErrFatal(err)
	byzcoin.ContractsFn[ContractValueID] = contractValueFromBytes
	byzcoin.ContractsFn[ContractCoinID] = contractCoinFromBytes
	byzcoin.ContractsFn[ContractInsecureDarcID] = contractInsecureDarcFromBytes
}

// Service is only used to being able to store our contracts
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
}

// interface to byzcoin.Service
func (s *Service) byzService() *byzcoin.Service {
	return s.Service(byzcoin.ServiceName).(*byzcoin.Service)
}

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	return s, nil
}
