package unicore

import (
	"crypto/sha256"
	"errors"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
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
	bs *byzcoin.Service
}

// GetState will return the state of the given smart contract
func (s *Service) GetState(req *GetStateRequest) (*GetStateReply, error) {
	rst, err := s.bs.GetReadOnlyStateTrie(req.ByzCoinID)
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	h.Write(req.InstanceID)
	vid := h.Sum(nil)

	v, _, cid, _, err := rst.GetValues(vid)
	if err != nil {
		return nil, err
	}

	if cid != contractName {
		// otherwise this gives access to any instance
		return nil, errors.New("mismatch contract ID")
	}

	return &GetStateReply{Value: v}, nil
}

func contractFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &contract{}
	return c, nil
}

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		bs:               c.Service(byzcoin.ServiceName).(*byzcoin.Service),
	}

	if err := s.RegisterHandlers(s.GetState); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}

	byzcoin.RegisterContract(s, contractName, contractFromBytes)
	return s, nil
}
