package unicore

import (
	"log"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3"
)

// ServiceName is the identifier of the service
const ServiceName = "UnicoreSmartContract"

const contractName = "unicore"

var sid onet.ServiceID

func init() {
	var err error
	sid, err = onet.RegisterNewService(ServiceName, newService)
	if err != nil {
		log.Fatal(err)
	}
}

// Service is the Unicore service that will spawn an invoke native smart contract
type Service struct {
	*onet.ServiceProcessor
}

func contractFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &contract{}
	return c, nil
}

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}

	byzcoin.RegisterContract(s, contractName, contractFromBytes)
	return s, nil
}
