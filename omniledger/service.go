package omniledger

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"errors"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	//	"github.com/dedis/cothority/byzcoin/darc/expression"
	"fmt"
	lib "github.com/dedis/cothority/omniledger/lib"
	"github.com/dedis/cothority/skipchain"
	"sync"
	"time"

	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// OmniLedgerID contains the service id
var OmniLedgerID onet.ServiceID

func init() {
	var err error
	OmniLedgerID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessage(&storage{})
}

// Service is our OmniLedger-service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
	contracts map[string]bc.ContractFn
	storage   *storage
}

// storageID reflects the data we're storing - we could store more
// than one structure.
var storageID = []byte("OmniLedger")

// storage is used to save our data.
type storage struct {
	Count int
	sync.Mutex
}

type CreateOmniLedger struct {
	Version      bc.Version
	Roster       onet.Roster
	ShardCount   int
	EpochSize    time.Duration
	IBGenesisMsg *bc.CreateGenesisBlock
	//ShardGenesisMsg  *bc.CreateGenesisBlock
	OwnerID darc.Identity
	//SpawnInstruction *bc.Instruction
	SpawnTx   *bc.ClientTransaction
	Timestamp time.Time
}

type CreateOmniLedgerResponse struct {
	Version              bc.Version
	ShardRoster          []onet.Roster
	IDSkipBlock          *skipchain.SkipBlock  // Genesis block of the identity ledger
	ShardBlocks          []skipchain.SkipBlock // Genesis block of each shard
	GenesisDarc          darc.Darc
	Owner                darc.Signer
	OmniledgerInstanceID bc.InstanceID
}

type NewEpoch struct {
	IBID     skipchain.SkipBlockID
	IBRoster onet.Roster
	//IBDarcID     darc.Darc
	ShardIDs []skipchain.SkipBlockID
	//ShardDarcIDs []darc.Darc
	//ShardRosters []onet.Roster
	Owner         darc.Signer
	OLInstanceID  bc.InstanceID
	Timestamp     time.Time
	ReqNewEpochTx *bc.ClientTransaction
}

type NewEpochResponse struct {
	IBRoster         onet.Roster
	ReqNewEpochProof *bc.Proof
	//ShardRosters []onet.Roster
}

func (s *Service) CreateOmniLedger(req *CreateOmniLedger) (*CreateOmniLedgerResponse, error) {
	if err := checkCreateOmniLedger(req); err != nil {
		return nil, err
	}

	c, ibRep, err := bc.NewLedger(req.IBGenesisMsg, false)
	if err != nil {
		return nil, err
	}

	fmt.Println("-------- PRINT1 ---------")

	if _, err := c.AddTransactionAndWait(*req.SpawnTx, 2); err != nil {
		return nil, err
	}

	fmt.Println("-------- PRINT2 ---------")

	id := req.SpawnTx.Instructions[0].DeriveID("")
	gpr, err := c.GetProof(id.Slice())
	if err != nil {
		return nil, err
	}

	if !gpr.Proof.InclusionProof.Match(id.Slice()) {
		return nil, errors.New("no association found for the proof")
	}

	cc := &lib.ChainConfig{}
	err = gpr.Proof.VerifyAndDecode(cothority.Suite, ContractOmniledgerEpochID, cc)
	if err != nil {
		return nil, err
	}

	shardRosters := cc.ShardRosters

	// Create shards using byzcoin
	// Create the messages -> Create the ledger of each shard
	msgs := make([]*bc.CreateGenesisBlock, req.ShardCount)
	for i := 0; i < req.ShardCount; i++ {
		// TODO: Add "invoke:newepoch"
		msg, err := byzcoin.DefaultGenesisMsg(req.Version, &shardRosters[i], []string{"spawn:darc", "invoke:newepoch"}, req.OwnerID)
		if err != nil {
			return nil, err
		}
		msgs[i] = msg
	}

	ids := make([]skipchain.SkipBlock, req.ShardCount)
	for i := 0; i < req.ShardCount; i++ {
		_, rep, err := bc.NewLedger(msgs[i], false)
		if err != nil {
			return nil, err
		}
		ids[i] = *rep.Skipblock
	}

	// Build reply
	reply := &CreateOmniLedgerResponse{
		Version:              req.Version,
		ShardRoster:          shardRosters,
		IDSkipBlock:          ibRep.Skipblock,
		ShardBlocks:          ids,
		OmniledgerInstanceID: id,
		//GenesisDarc: d,
		//Owner:       owner,
	}

	return reply, nil
}

func checkCreateOmniLedger(req *CreateOmniLedger) error {
	if len(req.Roster.List) < 1 {
		return errors.New("Empty roster")
	}

	if req.ShardCount < 1 {
		return errors.New("Null or negative number of shards")
	}

	if req.EpochSize < 1 {
		return errors.New("Null or negative epoch size")
	}

	if 4*req.ShardCount > len(req.Roster.List) {
		return errors.New("Not enough validators per shard")
	}

	if req.IBGenesisMsg == nil {
		return errors.New("Requires a genesis message")
	}

	if len(req.SpawnTx.Instructions) < 1 || req.SpawnTx.Instructions[0].Spawn == nil {
		return errors.New("Requires a spawn instruction")
	}

	if len(req.SpawnTx.Instructions[0].Signatures) < 1 {
		return errors.New("Spawn instruction must be signed")
	}

	return nil
}

func (s *Service) NewEpoch(req *NewEpoch) (*NewEpochResponse, error) {
	reply := &NewEpochResponse{}

	ibClient := bc.NewClient(req.IBID, req.IBRoster)

	_, err := ibClient.AddTransactionAndWait(*req.ReqNewEpochTx, 5)
	if err != nil {
		return nil, err
	}

	reqNewEpochInstrID := req.ReqNewEpochTx.Instructions[0].DeriveID("")
	gpr, err := ibClient.GetProof(reqNewEpochInstrID.Slice())
	if err != nil {
		return nil, err
	}

	// Send back proof
	reply.ReqNewEpochProof = &gpr.Proof

	return reply, nil
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real deployments.
func newService(c *onet.Context) (onet.Service, error) {
	// Create the service struct
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		contracts:        make(map[string]bc.ContractFn),
	}

	// Register handlers (i.e. methods the service will call must have signature func({}interface) ({}interface, error))
	if err := s.RegisterHandlers(s.CreateOmniLedger); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	if err := s.RegisterHandlers(s.NewEpoch); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	// Register processor function (handles certain message types, e.g. ViewChangeReq) if necessary
	// Register contracts

	// TODO: Use byzcoin.RegisterContract instead
	bc.RegisterContract(c, ContractOmniledgerEpochID, contractOmniledgerEpochFromBytes)
	//bc.RegisterContract(c, ContractNewEpochID, s.ContractNewEpoch)

	// Register verification
	// Register protocols
	// Register skipchain callbacks + enable view change
	// Register view-change cosi protocols
	// Start all chains

	return s, nil
}
