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
	"time"
)

var ContractConfigID = "config"

// ConfigInstanceID represents the 0-id of the configuration instance.
var ConfigInstanceID = bc.InstanceID{}

var ContractNewEpochID = "newepoch"

type ChainConfig struct {
	Roster     onet.Roster
	ShardCount int
	EpochSize  time.Duration
}

// ContractConfig ...
func (s *Service) ContractConfig(cdb bc.CollectionView, inst bc.Instruction,
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

	// Create ChainConfig struct to store
	config := &ChainConfig{
		Roster:     roster,
		ShardCount: shardCount,
		EpochSize:  epochSize,
	}

	// Check sanity of config

	// Encode the config
	configBuf, err := protobuf.Encode(&config)
	if err != nil {
		return
	}

	// Return state changes
	id := d.GetBaseID()
	return []bc.StateChange{
		bc.NewStateChange(bc.Create, ConfigInstanceID, ContractConfigID, configBuf, id),
	}, coins, nil
}

func invokeContractConfig(cdb bc.CollectionView, inst bc.Instruction, coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {

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
