package bevm

import (
	"errors"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

func init() {
	// Ethereum starts goroutines for caching transactions, and never terminates them
	log.AddUserUninterestingGoroutine("go-ethereum/core.(*txSenderCacher).cache")

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
		return nil, errors.New("error registering BEvm contract: " + err.Error())
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
		return nil, errors.New("error registering BEvmValue contract: " + err.Error())
	}

	return s, nil
}
