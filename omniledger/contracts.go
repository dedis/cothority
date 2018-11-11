package omniledger

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/dedis/cothority"
	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/darc"
	lib "github.com/dedis/cothority/omniledger/lib"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	"math/rand"
	"sort"
	"time"
)

const VALID_TIME_WINDOW = time.Second * 60

var ContractOmniledgerEpochID = "omniledgerepoch"

//var ContractNewEpochID = "newepoch"

// ConfigInstanceID represents the 0-id of the configuration instance.
var ConfigInstanceID = bc.InstanceID{}

// ContractOmniledgerEpoch ...
func (s *Service) ContractOmniledgerEpoch(cdb bc.CollectionView, inst bc.Instruction,
	coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {

	switch inst.GetType() {
	case bc.SpawnType:
		return spawnOmniledgerEpoch(cdb, inst, coins)
	case bc.InvokeType:
		return invokeOmniledgerEpoch(cdb, inst, coins)
	default:
		return nil, coins, errors.New("unsupported instruction type")
	}
}

func spawnOmniledgerEpoch(cdb bc.CollectionView, inst bc.Instruction, coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {
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
	epochSizeDecoded, err := binary.ReadVarint(bytes.NewBuffer(epochSizeBuf))
	if err != nil {
		log.Error("couldn't decode epoch size")
		return nil, coins, err
	}
	epochSize := time.Duration(int32(epochSizeDecoded)) * time.Millisecond

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

func invokeOmniledgerEpoch(cdb bc.CollectionView, inst bc.Instruction, coins []bc.Coin) (sc []bc.StateChange, c []bc.Coin, err error) {

	if inst.Invoke.Command == "request_new_epoch" {
		tsBuf := inst.Spawn.Args.Search("timestamp")
		ts := time.Unix(int64(binary.BigEndian.Uint64(tsBuf)), 0)
		if !checkValidTime(ts, time.Second*60) {
			return nil, coins, errors.New("Client timestamp is too different from node's clock")
		}

		buf, _, darcID, err := cdb.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return nil, coins, err
		}

		cc := &lib.ChainConfig{}
		if buf != nil {
			err = protobuf.Decode(buf, cc)
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

	return nil, coins, nil
}

// ContractNewEpoch ...
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
	//perm := rand.Perm(len(roster.List))

	perm := make([]int, len(roster.List))
	for i := 0; i < len(roster.List); i++ {
		perm[i] = i
	}

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

	// Compute the sorted list of keys
	keys := make([]int, 0)
	for k := range m {
		keys = append(keys, k)
	}

	sort.Ints(keys)

	// Group validators by shard index
	idGroups := make([][]*network.ServerIdentity, shardCount)

	for k := range keys {
		v := m[k]
		idGroups[v] = append(idGroups[v], roster.List[k])
	}

	// Create shard rosters
	shardRosters := make([]onet.Roster, shardCount)
	for ind, ids := range idGroups {
		temp := onet.NewRoster(ids)
		shardRosters[ind] = *temp
	}

	return shardRosters
}

func checkValidTime(t time.Time, window time.Duration) bool {
	diff := time.Since(t)
	if diff < 0 {
		diff *= -1
	}

	return diff.Seconds() <= window.Seconds()
}
