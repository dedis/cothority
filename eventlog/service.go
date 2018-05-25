package eventlog

import (
	"errors"

	"github.com/dedis/student_18_omniledger/omniledger/collection"
	omniledger "github.com/dedis/student_18_omniledger/omniledger/service"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
)

// ServiceName is the service name for the EventLog service.
var ServiceName = "EventLog"

var sid onet.ServiceID

var indexKey omniledger.ObjectID

func init() {
	var err error
	sid, err = onet.RegisterNewService(ServiceName, newService)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: This is horrible, we'll be somehow using better well known keys later. Maybe.
	indexKey.DarcID = omniledger.ZeroDarc
	var keyNonce = omniledger.ZeroNonce
	copy(keyNonce[:], []byte("index"))
	indexKey.InstanceID = keyNonce
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
		GenesisDarc: req.Owner,
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
	req2 := &omniledger.AddTxRequest{
		Version:     omniledger.CurrentVersion,
		SkipchainID: req.SkipchainID,
		Transaction: req.Transaction,
	}
	_, err := s.omni.AddTransaction(req2)
	if err != nil {
		return nil, err
	}
	return &LogResponse{}, nil
}

const contractName = "eventlog"

// contractFunction is the function that runs to process a transaction of
// type "eventlog"
func (s *Service) contractFunction(cdb collection.Collection, tx omniledger.Instruction, c []omniledger.Coin) ([]omniledger.StateChange, []omniledger.Coin, error) {
	if tx.Spawn == nil {
		return nil, nil, errors.New("expected a spawn tx")
	}
	// TODO: Disallow all non-Spwan tx types.

	// This is not strictly required, because since we know we are
	// a spawn, we know the contract comes directly from
	// tx.Spawn.ContractID.
	cid, _, err := tx.GetContractState(cdb)
	if err != nil {
		return nil, nil, err
	}

	event := tx.Spawn.Args.Search("event")
	if event == nil {
		return nil, nil, errors.New("expected a named argument of \"event\"")
	}

	sc := []omniledger.StateChange{
		omniledger.NewStateChange(omniledger.Create, tx.ObjectID, cid, event),
	}

	r, err := cdb.Get(indexKey.Slice()).Record()
	if err != nil {
		return nil, nil, err
	}
	v, err := r.Values()
	if err == nil {
		// If we have a previous value, and it's the correct type, append the new event to it.
		if newval, ok := v[0].([]byte); ok {
			newval = append(newval, tx.ObjectID.Slice()...)
			return append(sc, omniledger.NewStateChange(omniledger.Update, indexKey, cid, newval)), nil, nil
		}
	}
	// Otherwise make a new key for the index.
	return append(sc, omniledger.NewStateChange(omniledger.Create, indexKey, cid, tx.ObjectID.Slice())), nil, nil
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
