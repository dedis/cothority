package eventlog

import (
	"errors"
	"time"

	"github.com/dedis/protobuf"
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
	omni          *omniledger.Service
	blockInterval time.Duration
}

const defaultBlockInterval = 5 * time.Second

// waitForBlock is for use in tests; it will sleep long enough to be sure that
// a block has been created.
func (s *Service) waitForBlock() { time.Sleep(5 * s.blockInterval) }

// Init will create a new event log. Logs will be accepted
// from the signers mentioned in the request.
func (s *Service) Init(req *InitRequest) (*InitResponse, error) {
	cg := &omniledger.CreateGenesisBlock{
		Version:       omniledger.CurrentVersion,
		GenesisDarc:   req.Owner,
		Roster:        req.Roster,
		BlockInterval: s.blockInterval,
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
	if tx.Delete != nil {
		return nil, nil, errors.New("delete tx not allowed")
	}
	if tx.Invoke != nil {
		return nil, nil, errors.New("invoke tx not allowed")
	}

	if tx.Spawn == nil {
		return nil, nil, errors.New("expected a spawn tx")
	}

	// This is not strictly required, because since we know we are
	// a spawn, we know the contract comes directly from
	// tx.Spawn.ContractID.
	cid, _, err := tx.GetContractState(cdb)
	if err != nil {
		return nil, nil, err
	}

	evBuf := tx.Spawn.Args.Search("event")
	if evBuf == nil {
		return nil, nil, errors.New("expected a named argument of \"event\"")
	}

	// Check the timestamp of the event: it should never be in the future, and it
	// should not be more than 10 blocks in the past. (Why 10? Because it works.
	// But it would be nice to have a better way to hold down the window size, and
	// to be able to reason about it better.)
	//
	// TODO: Adaptive window size based on recently observed block latencies?
	event := &Event{}
	err = protobuf.Decode(evBuf, event)
	if err != nil {
		return nil, nil, err
	}
	when := time.Unix(0, event.When)
	now := time.Now()
	//log.LLvl2("when", when, "now", now, "sub", now.Sub(when), "int", s.blockInterval)
	if when.Before(now.Add(-10 * s.blockInterval)) {
		return nil, nil, errors.New("event timestamp too long ago")
	}
	if when.After(now) {
		return nil, nil, errors.New("event timestamp is in the future")
	}

	sc := []omniledger.StateChange{
		omniledger.NewStateChange(omniledger.Create, tx.ObjectID, cid, evBuf),
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
