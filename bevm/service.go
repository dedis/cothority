package bevm

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

func init() {
	_, err := onet.RegisterNewService(ContractBEvmID, newServiceBEvm)
	log.ErrFatal(err)
	_, err = onet.RegisterNewService(ContractBEvmValueID, newServiceBEvmValue)
	log.ErrFatal(err)
}

// Service structure for BEvm contracts
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
}

func newServiceBEvm(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}

	err := byzcoin.RegisterContract(c, ContractBEvmID, contractBEvmFromBytes)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func newServiceBEvmValue(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}

	// BEvmValue does not support explicit creation, so we can pass nil
	err := byzcoin.RegisterContract(c, ContractBEvmValueID, nil)
	if err != nil {
		return nil, err
	}

	return s, nil
}
