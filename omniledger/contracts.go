package omniledger

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/dedis/cothority"
	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/darc"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	"math/rand"
	"time"
)

var ContractOmniledgerEpochID = "omniledgerepoch"
var ContractNewEpochID = "newepoch"

// ConfigInstanceID represents the 0-id of the configuration instance.
var ConfigInstanceID = bc.InstanceID{}

type ChainConfig struct {
	Roster     onet.Roster
	ShardCount int
	EpochSize  time.Duration
	Timestamp  time.Time
}

// ContractOmniledgerEpoch ...
func (s *Service) ContractOmniledgerEpoch(cdb bc.CollectionView, inst bc.Instruction,
	coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {

	switch inst.GetType() {
	case bc.SpawnType:
		return spawnContractConfig(cdb, inst, coins)
	case bc.InvokeType:
		return invokeContractConfig(cdb, inst, coins)
	default:
		return nil, coins, errors.New("unsupported instruction type")
	}
}

func spawnContractConfig(cdb bc.CollectionView, inst bc.Instruction, coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {
	// Decode darc and verify it
	darcBuf := inst.Spawn.Args.Search("darc")
	d, err := darc.NewFromProtobuf(darcBuf)
	if err != nil {
		log.Error("couldn't decode darc")
		return
	}
	if d.Rules.Count() == 0 {
		return nil, nil, errors.New("don't accept darc with empty rules")
	}
	if err = d.Verify(true); err != nil {
		log.Error("couldn't verify darc")
		return
	}

	// Get arguments from the instruction's arguments (#shard, epoch-size)
	shardCountBuf := inst.Spawn.Args.Search("shardCount")
	shardCountDecoded, err := binary.ReadVarint(bytes.NewBuffer(shardCountBuf))
	if err != nil {
		log.Error("couldn't decode shard count")
		return
	}
	shardCount := int(shardCountDecoded)

	epochSizeBuf := inst.Spawn.Args.Search("epochSize")
	epochSizeDecoded, err := binary.ReadVarint(bytes.NewBuffer(epochSizeBuf))
	if err != nil {
		log.Error("couldn't decode epoch size")
	}
	epochSize := time.Duration(int32(epochSizeDecoded)) * time.Millisecond

	// Get roster from instruction's arguments
	rosterBuf := inst.Spawn.Args.Search("roster")
	roster := onet.Roster{}
	err = protobuf.DecodeWithConstructors(rosterBuf, &roster, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return
	}

	// Do sharding
	shardRosters := sharding(&roster, shardCount, int64(binary.BigEndian.Uint64(inst.DeriveID("").Slice())))

	// Create ChainConfig struct to store data on the chain
	config := &ChainConfig{
		Roster:     roster,
		ShardCount: shardCount,
		EpochSize:  epochSize,
		Timestamp:  time.Now(),
	}

	// TODO: Check sanity of config

	// Encode the config
	configBuf, err := protobuf.Encode(&config)
	if err != nil {
		return
	}

	// Encode the rosters
	shardRostersBuf, err := protobuf.Encode(shardRosters)
	if err != nil {
		return
	}

	// Return state changes
	darcID := d.GetBaseID()
	return []bc.StateChange{
		bc.NewStateChange(bc.Create, ConfigInstanceID, ContractOmniledgerEpochID, configBuf, darcID),
		bc.NewStateChange(bc.Create, ConfigInstanceID, ContractOmniledgerEpochID, shardRostersBuf, darcID),
	}, coins, nil
}

func invokeContractConfig(cdb bc.CollectionView, inst bc.Instruction, coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {

	if inst.Invoke.Command == "request_new_epoch" {
		// No args
		// Checks that it is indeed time for a new epoch
		// Must retrieve current config to get the epoch_duration parameter

		// Updates the omniledger epoch instance with the latest roster from the IB (which is a BC config)
		// > Get the latest roster from the IB
		// > Create a new ChainConfig with the new Roster
		// > Return these changes

		// After the update, the service needs to take its proof and send it to the BC config using invoke:newEpoch (called for the shards)

		// Create state change that includes copy of the new fixed roster to be used

	}

	return
}

// ContractNewEpoch ...
// TODO: Complete the function
// The id of the previous block is in the Instruction?
// add view to signature
// For the moment, seed is an instruction's argument
func (s *Service) ContractNewEpoch(cdb bc.CollectionView, inst bc.Instruction,
	coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {

	switch inst.GetType() {
	case bc.SpawnType:
		return
	case bc.InvokeType:
		return
	default:
		return nil, coins, errors.New("unsupported instruction type")
	}
}

func sharding(roster *onet.Roster, shardCount int, seed int64) []onet.Roster {
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
