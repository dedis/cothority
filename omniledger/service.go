package omniledger

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"encoding/binary"
	"errors"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	//	"github.com/dedis/cothority/byzcoin/darc/expression"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/protobuf"
	"sync"
	"time"

	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/darc"
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
	Version    bc.Version
	Roster     onet.Roster
	ShardCount int
	EpochSize  time.Duration
}

type CreateOmniLedgerResponse struct {
	Version     bc.Version
	ShardRoster []onet.Roster
	IDSkipBlock *skipchain.SkipBlock  // Genesis block of the identity ledger
	ShardBlocks []skipchain.SkipBlock // Genesis block of each shard
	GenesisDarc darc.Darc
	Owner       darc.Signer
}

type NewEpoch struct {
	//
}

type NewEpochResponse struct {
}

func (s *Service) CreateOmniLedger(req *CreateOmniLedger) (*CreateOmniLedgerResponse, error) {

	if err := checkCreateOmniLedger(req); err != nil {
		return nil, err
	}

	// Create owner
	owner := darc.NewSignerEd25519(nil, nil)

	// Create the IB using byzcoin service (will contain genesis block + darc)
	msg, err := byzcoin.DefaultGenesisMsg(req.Version,
		&req.Roster, []string{"spawn:darc"}, owner.Identity())
	if err != nil {
		return nil, err
	}
	c, rep, err := bc.NewLedger(msg, false)
	if err != nil {
		return nil, err
	}

	tx := byzcoin.ClientTransaction{
		Instructions: make([]byzcoin.Instruction, 1),
	}

	darc := msg.GenesisDarc
	//darc.Rules.AddRule("spawn:omniledgerepoch", expression.Expr(owner.Identity().String()))
	darc.Rules.AddRule("spawn:omniledgerepoch", darc.Rules.GetSignExpr())

	darcBuf, err := protobuf.Encode(&darc)
	if err != nil {
		return nil, err
	}

	instr1 := byzcoin.Instruction{
		InstanceID: bc.NewInstanceID(darc.BaseID),
		Nonce:      bc.GenNonce(),
		Index:      0,
		Length:     1,
		Invoke: &bc.Invoke{
			Command: "evolve",
			Args: []bc.Argument{
				bc.Argument{Name: "darc", Value: darcBuf},
			},
		},
	}
	instr1.SignBy(darc.GetBaseID(), owner)

	tx.Instructions[0] = instr1
	if _, err := c.AddTransactionAndWait(tx, 10); err != nil {
		return nil, err
	}

	// Store parameters (#shard and epoch-size) in the identity ledger
	scBuff := make([]byte, 8)
	binary.PutVarint(scBuff, int64(req.ShardCount))

	esBuff := make([]byte, 8)
	binary.PutVarint(scBuff, int64(req.EpochSize))

	instr2 := byzcoin.Instruction{
		InstanceID: bc.NewInstanceID(darc.BaseID),
		Nonce:      bc.GenNonce(),
		Index:      0,
		Length:     1,
		Spawn: &bc.Spawn{
			ContractID: ContractOmniledgerEpochID,
			Args: []bc.Argument{
				bc.Argument{Name: "darc", Value: darcBuf},
				bc.Argument{Name: "shardCount", Value: scBuff},
				bc.Argument{Name: "epochSize", Value: esBuff}},
		},
	}
	instr2.SignBy(darc.GetID(), owner)

	tx.Instructions[0] = instr2
	if _, err := c.AddTransactionAndWait(tx, 10); err != nil {
		return nil, err
	}

	// Do sharding to generate ShardRoster
	/*
		seed, err := binary.ReadVarint(bytes.NewBuffer(c.ID))
		if err != nil {
			log.Error("couldn't decode skipblock hash")
			return nil, err
		}
		shardRosters := sharding(&req.Roster, req.ShardCount, seed)
	*/

	// Get shardRosters from proof
	id := tx.Instructions[0].Hash()
	gpr, err := c.GetProof(id)
	if err != nil {
		return nil, err
	}

	if !gpr.Proof.InclusionProof.Match() {
		return nil, errors.New("no association found for the proof")
	}

	var shardRosters []onet.Roster
	gpr.Proof.ContractValue(cothority.Suite, ContractOmniledgerEpochID, shardRosters)

	// Create shards using byzcoin
	// Create the messages -> Create the ledger of each shard
	msgs := make([]*bc.CreateGenesisBlock, req.ShardCount)
	for i := 0; i < req.ShardCount; i++ {
		msg, err := byzcoin.DefaultGenesisMsg(req.Version, &shardRosters[i], []string{"spawn:darc"}, owner.Identity())
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
		Version:     req.Version,
		ShardRoster: shardRosters,
		IDSkipBlock: rep.Skipblock,
		ShardBlocks: ids,
		GenesisDarc: darc,
		Owner:       owner,
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

	return nil
}

/*
func sharding(roster *onet.Roster, shardCount int, seed int64) []onet.Roster {
	rand.Seed(seed)
	perm := rand.Perm(len(roster.List))
	shardRosters := make([]onet.Roster, 0)
	shardSize := int64(math.Floor(float64(len(roster.List) / shardCount)))

	batches := make([][]int, 0)
	for len(perm) > shardCount {
		batches = append(batches, perm[0:shardSize])
		perm = perm[shardSize:]
	}
	batches = append(batches, perm)

	for i := 0; i < len(batches)-1; i++ {
		batch := batches[i]
		serverIDs := make([]*network.ServerIdentity, 0)
		for j := 0; j < len(batch); j++ {
			serverIDs = append(serverIDs, roster.List[batch[j]])
		}
		// TODO: new roster method (onet.newroster)
		shardRosters = append(shardRosters, onet.Roster{
			// ID?
			List: serverIDs,
			// Aggregate?
		})
	}

	return shardRosters
}
*/

/*func sharding(roster *onet.Roster, shardCount int, seed int64) []onet.Roster {
	rand.Seed(seed)
	perm := rand.Perm(len(roster.List))

	// Build map: validator index to shard index
	m := make(map[int]int)
	c := 0
	for _, p := range perm {
		if c == shardCount {
			c = 0
		}

		m[p] = c
		c++
	}

	// Group validators by shard index
	idGroups := make([][]*network.ServerIdentity, shardCount)
	for k, v := range m {
		idGroups[v] = append(idGroups[v], roster.List[k])
	}

	// Create shard rosters
	shardRosters := make([]onet.Roster, shardCount)
	for ind, ids := range idGroups {
		shardRosters[ind] = *onet.NewRoster(ids)
	}

	return shardRosters
}
*/

// NewEpoch
func (s *Service) NewEpoch(req *NewEpoch) (*NewEpochResponse, error) {
	// > Connect to the IB via a client, requires a SkipBlockID -> need to store the genesis block of the IB in the service?
	// Send an invoke:requestNewEpoch
	/*byzcoin.NewClient()
	invoke := &bc.Invoke{
		Command: "request_new_epoch",
	}*/

	return nil, nil
}

// AddNode

// RemoveNode

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
// If you use CreateProtocolOnet, this will not be called, as the Onet will
// instantiate the protocol on its own. If you need more control at the
// instantiation of the protocol, use CreateProtocolService, and you can
// give some extra-configuration to your protocol in here.
/*func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3("Not templated yet")
	return nil, nil
}*/

// saves all data.
/*func (s *Service) save() {
	s.storage.Lock()
	defer s.storage.Unlock()
	err := s.Save(storageID, s.storage)
	if err != nil {
		log.Error("Couldn't save data:", err)
	}
}*/

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
/*func (s *Service) tryLoad() error {
	s.storage = &storage{}
	msg, err := s.Load(storageID)
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	var ok bool
	s.storage, ok = msg.(*storage)
	if !ok {
		return errors.New("Data of wrong type")
	}
	return nil
}*/

func (s *Service) registerContract(contractID string, c bc.ContractFn) error {
	s.contracts[contractID] = c
	return nil
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
	// Register processor function (handles certain message types, e.g. ViewChangeReq) if necessary
	// Register contracts
	s.registerContract(ContractOmniledgerEpochID, s.ContractOmniledgerEpoch)
	s.registerContract(ContractNewEpochID, s.ContractNewEpoch)

	// Register verification
	// Register protocols
	// Register skipchain callbacks + enable view change
	// Register view-change cosi protocols
	// Start all chains

	return s, nil
}
