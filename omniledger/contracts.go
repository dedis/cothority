package omniledger

import (
	"bytes"
	"encoding/binary"
	"errors"
	//"fmt"
	"github.com/dedis/cothority"
	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	lib "github.com/dedis/cothority/omniledger/lib"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	//"math/rand"
	//"sort"
	"time"
)

const VALID_TIME_WINDOW = time.Second * 60

var ContractOmniledgerEpochID = "omniledgerepoch"

//var ContractNewEpochID = "newepoch"

//var ConfigInstanceID = bc.InstanceID{}

type contractOmniledgerEpoch struct {
	bc.BasicContract
	lib.ChainConfig
}

func contractOmniledgerEpochFromBytes(in []byte) (bc.Contract, error) {
	c := &contractOmniledgerEpoch{}
	err := protobuf.DecodeWithConstructors(in, &c.ChainConfig, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *contractOmniledgerEpoch) Spawn(rst bc.ReadOnlyStateTrie, inst bc.Instruction, coins []bc.Coin) (sc []bc.StateChange, cout []bc.Coin, err error) {
	cout = coins

	darcBuf := inst.Spawn.Args.Search("darc")
	d, err := darc.NewFromProtobuf(darcBuf)
	if err != nil {
		log.Error("couldn't decode darc")
		return
	}
	if d.Rules.Count() == 0 {
		err = errors.New("don't accept darc with empty rules")
		return
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
	epochSize, err := lib.DecodeDuration(epochSizeBuf)
	if err != nil {
		log.Error("couldn't decode epoch size")
		return
	}

	tsBuf := inst.Spawn.Args.Search("timestamp")
	ts := time.Unix(int64(binary.BigEndian.Uint64(tsBuf)), 0)
	if !checkValidTime(ts, VALID_TIME_WINDOW) {
		err = errors.New("Client timestamp is too different from node's clock")
		return
	}

	// Get roster from instruction's arguments
	rosterBuf := inst.Spawn.Args.Search("roster")
	roster := &onet.Roster{}
	err = protobuf.DecodeWithConstructors(rosterBuf, roster, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		log.Error("Error while decoding constructors")
		return
	}

	// Do sharding
	shardRosters := lib.Sharding(roster, shardCount, int64(binary.BigEndian.Uint64(inst.DeriveID("").Slice())))

	// Create ChainConfig struct to store data on the chain
	config := &lib.ChainConfig{
		Roster:       roster,
		ShardCount:   shardCount,
		EpochSize:    epochSize,
		Timestamp:    ts,
		ShardRosters: shardRosters,
	}

	// TODO: Check sanity of config

	// Encode the config
	configBuf, err := protobuf.Encode(config)
	if err != nil {
		return
	}

	// Return state changes
	darcID := d.GetBaseID()
	sc = []bc.StateChange{
		bc.NewStateChange(bc.Create, inst.DeriveID(""), ContractOmniledgerEpochID, configBuf, darcID),
	}

	return
}

func (c *contractOmniledgerEpoch) Invoke(rst bc.ReadOnlyStateTrie, inst bc.Instruction, coins []bc.Coin) (sc []bc.StateChange, cout []bc.Coin, err error) {
	cout = coins

	switch inst.Invoke.Command {
	case "request_new_epoch":
		tsBuf := inst.Invoke.Args.Search("timestamp")
		ts := time.Unix(int64(binary.BigEndian.Uint64(tsBuf)), 0)
		if !checkValidTime(ts, time.Second*60) {
			err = errors.New("Client timestamp is too different from node's clock")
			return
		}

		var buf []byte
		var darcID darc.ID
		buf, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return
		}

		cc := &lib.ChainConfig{}
		if buf != nil {
			err = protobuf.DecodeWithConstructors(buf, cc, network.DefaultConstructors(cothority.Suite))
			if err != nil {
				return
			}
		}

		if ts.Sub(cc.Timestamp).Seconds() >= cc.EpochSize.Seconds() {
			// compute new shards
			seed := int64(binary.BigEndian.Uint64(inst.DeriveID("").Slice()))

			shardRosters := lib.Sharding(cc.Roster, cc.ShardCount, seed)

			// update chain config
			cc.Timestamp = ts
			cc.ShardRosters = shardRosters
			var ccBuf []byte
			ccBuf, err = protobuf.Encode(cc)
			if err != nil {
				return
			}

			// return changes
			sc = []bc.StateChange{
				bc.NewStateChange(bc.Update, inst.DeriveID(""), ContractOmniledgerEpochID, ccBuf, darcID),
			}
			log.Print("UPDATED ID BYZCOIN")

			return
		}

		return nil, coins, errors.New("Request new epoch failed, was called too soon")
	default:
		err = errors.New("unknown instruction type")
		return
	}
}

/*
// ContractOmniledgerEpoch ...
func (s *Service) ContractOmniledgerEpoch(cdb bc.ReadOnlyStateTrie, inst bc.Instruction, ctxHash []byte,
	coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {
	// Verify the darc signature if the config instance does not exist yet.

		pr, err := cdb.GetProof(ConfigInstanceID.Slice())
		if err != nil {
			return
		}
		ok, err := pr.Exists(ConfigInstanceID.Slice())
		if err != nil {
			return
		}
		if ok {
			err = inst.Verify(cdb, ctxHash)
			if err != nil {
				return
			}
		}


	switch inst.GetType() {
	case bc.SpawnType:
		return spawnOmniledgerEpoch(cdb, inst, coins)
	case bc.InvokeType:
		return invokeOmniledgerEpoch(cdb, inst, coins)
	default:
		return nil, coins, errors.New("unsupported instruction type")
	}
}*/

/*
func spawnOmniledgerEpoch(cdb bc.ReadOnlyStateTrie, inst bc.Instruction, coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {
	// Decode darc and verify it

	darcBuf := inst.Spawn.Args.Search("darc")
	d, err := darc.NewFromProtobuf(darcBuf)
	if err != nil {
		log.Error("couldn't decode darc")
		return nil, coins, err
	}
	if d.Rules.Count() == 0 {
		return nil, coins, errors.New("don't accept darc with empty rules")
	}
	if err = d.Verify(true); err != nil {
		log.Error("couldn't verify darc")
		return nil, coins, err
	}

	// Get arguments from the instruction's arguments (#shard, epoch-size)
	shardCountBuf := inst.Spawn.Args.Search("shardCount")
	shardCountDecoded, err := binary.ReadVarint(bytes.NewBuffer(shardCountBuf))
	if err != nil {
		log.Error("couldn't decode shard count")
		return nil, coins, err
	}
	shardCount := int(shardCountDecoded)

	epochSizeBuf := inst.Spawn.Args.Search("epochSize")
	epochSize, err := lib.DecodeDuration(epochSizeBuf)
	if err != nil {
		log.Error("couldn't decode epoch size")
		return nil, coins, err
	}

	tsBuf := inst.Spawn.Args.Search("timestamp")
	ts := time.Unix(int64(binary.BigEndian.Uint64(tsBuf)), 0)
	if !checkValidTime(ts, VALID_TIME_WINDOW) {
		return nil, coins, errors.New("Client timestamp is too different from node's clock")
	}

	// Get roster from instruction's arguments
	rosterBuf := inst.Spawn.Args.Search("roster")
	roster := &onet.Roster{}
	err = protobuf.DecodeWithConstructors(rosterBuf, roster, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		log.Error("Error while decoding constructors")
		return nil, coins, err
	}

	// Do sharding
	shardRosters := sharding(roster, shardCount, int64(binary.BigEndian.Uint64(inst.DeriveID("").Slice())))

	// Create ChainConfig struct to store data on the chain
	config := &lib.ChainConfig{
		Roster:       roster,
		ShardCount:   shardCount,
		EpochSize:    epochSize,
		Timestamp:    ts,
		ShardRosters: shardRosters,
	}

	// TODO: Check sanity of config

	// Encode the config
	configBuf, err := protobuf.Encode(config)
	if err != nil {
		return nil, coins, err
	}

	// Return state changes
	darcID := d.GetBaseID()
	return []bc.StateChange{
		bc.NewStateChange(bc.Create, inst.DeriveID(""), ContractOmniledgerEpochID, configBuf, darcID),
	}, coins, nil
}
*/

/*
func invokeOmniledgerEpoch(cdb bc.ReadOnlyStateTrie, inst bc.Instruction, coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {
	if inst.Invoke.Command == "request_new_epoch" {
		tsBuf := inst.Invoke.Args.Search("timestamp")
		ts := time.Unix(int64(binary.BigEndian.Uint64(tsBuf)), 0)
		if !checkValidTime(ts, time.Second*60) {
			return nil, coins, errors.New("Client timestamp is too different from node's clock")
		}

		buf, _, _, darcID, err := cdb.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return nil, coins, err
		}

		cc := &lib.ChainConfig{}
		if buf != nil {
			err = protobuf.DecodeWithConstructors(buf, cc, network.DefaultConstructors(cothority.Suite))
			if err != nil {
				return nil, coins, err
			}
		}

		if ts.Sub(cc.Timestamp).Seconds() >= cc.EpochSize.Seconds() {
			// compute new shards
			seed := int64(binary.BigEndian.Uint64(inst.DeriveID("").Slice()))
			shardRosters := sharding(cc.Roster, cc.ShardCount, seed)

			// update chain config
			cc.Timestamp = ts
			cc.ShardRosters = shardRosters
			ccBuf, err := protobuf.Encode(cc)
			if err != nil {
				return nil, coins, err
			}

			// return changes
			return []bc.StateChange{
				bc.NewStateChange(bc.Update, inst.DeriveID(""), ContractOmniledgerEpochID, ccBuf, darcID),
			}, coins, nil
		}

		return nil, coins, errors.New("Request new epoch failed, was called too soon")
	}

	return nil, coins, errors.New("unknown instruction type")
}
*/

// ContractNewEpoch ...
func (s *Service) ContractNewEpoch(cdb bc.ReadOnlyStateTrie, inst bc.Instruction,
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

func checkValidTime(t time.Time, window time.Duration) bool {
	diff := time.Since(t)
	if diff < 0 {
		diff *= -1
	}

	return diff.Seconds() <= window.Seconds()
}
