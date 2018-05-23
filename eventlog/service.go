package eventlog

import (
	"errors"

	"github.com/dedis/student_18_omniledger/omniledger/collection"
	omniledger "github.com/dedis/student_18_omniledger/omniledger/service"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
)

var ServiceName = "EventLog"
var sid onet.ServiceID

func init() {
	var err error
	sid, err = onet.RegisterNewService(ServiceName, newService)
	if err != nil {
		log.Fatal(err)
	}
}

// Service is the EventLog service.
type Service struct {
	*onet.ServiceProcessor
	omni *omniledger.Service
}

// Init will create a new event log. Logs will be accepted
// from the signers mentioned in the request.
func (s *Service) Init(req *InitRequest) (*InitResponse, error) {
	cg := &omniledger.CreateGenesisBlock{
		Version:     omniledger.CurrentVersion,
		GenesisDarc: req.Writer,
		Roster:      req.Roster,
	}
	cgr, err := s.omni.CreateGenesisBlock(cg)
	if err != nil {
		return nil, err
	}

	return &InitResponse{
		ID: cgr.Skipblock.Hash,
	}, nil
}

// Log will create a new event log entry.
func (s *Service) Log(req *LogRequest) (*LogResponse, error) {
	return nil, errors.New("not impl")
}

const contractName = "eventlog"

// contractFunction is the function that runs to process a transaction of
// type "eventlog"
func (s *Service) contractFunction(cdb collection.Collection, tx omniledger.Instruction, c []omniledger.Coin) ([]omniledger.StateChange, []omniledger.Coin, error) {
	return nil, nil, errors.New("not impl")
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real
// deployments.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		omni:             c.Service(omniledger.ServiceName).(*omniledger.Service),
	}
	if err := s.RegisterHandlers(s.Init, s.Log); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}

	omniledger.RegisterContract(s, contractName, s.contractFunction)
	return s, nil
}
