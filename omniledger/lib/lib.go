package lib

import (
	"bytes"
	"encoding/binary"

	"github.com/dedis/onet"
	//"github.com/dedis/onet/log"
	"math/rand"
	"time"

	"github.com/dedis/onet/network"
)

type ChainConfig struct {
	Roster       *onet.Roster
	ShardCount   int
	EpochSize    time.Duration
	Timestamp    time.Time
	ShardRosters []onet.Roster
}

func ChangeRoster(oldRoster, newRoster onet.Roster, oldMap, newMap map[network.ServerIdentityID]bool) (onet.Roster, map[network.ServerIdentityID]bool, map[network.ServerIdentityID]bool, bool) {
	intermList := append([]*network.ServerIdentity{}, oldRoster.List...)
	newList := newRoster.List

	if oldMap == nil {
		oldMap = make(map[network.ServerIdentityID]bool)
		for _, o := range intermList {
			oldMap[o.ID] = true
		}
	}

	// Add new element of newRoster to OldRoster, one at the time
	for _, n := range newList {
		if _, ok := oldMap[n.ID]; !ok {
			intermList = append(intermList, n)
			oldMap[n.ID] = true
			return *onet.NewRoster(intermList), oldMap, newMap, true
		}
	}

	if newMap == nil {
		newMap = make(map[network.ServerIdentityID]bool)
		for _, n := range newList {
			newMap[n.ID] = true
		}
	}

	// Remove old element of oldRoster, one at the time
	for i, o := range intermList {
		if _, ok := newMap[o.ID]; !ok {
			intermList = append(intermList[:i], intermList[i+1:]...)
			return *onet.NewRoster(intermList), oldMap, newMap, true
		}
	}

	return oldRoster, oldMap, newMap, false
}

func EncodeDuration(d time.Duration) []byte {
	durationInNs := int64(d * time.Nanosecond)
	tBuf := make([]byte, 8)
	binary.PutVarint(tBuf, durationInNs)

	return tBuf
}

func DecodeDuration(dBuf []byte) (time.Duration, error) {
	decoded, err := binary.ReadVarint(bytes.NewBuffer(dBuf))
	if err != nil {
		return time.Duration(0), err
	}

	duration := time.Duration(int64(decoded)) * time.Nanosecond

	return duration, nil
}

func Sharding(roster *onet.Roster, shardCount int, seed int64) []onet.Roster {
	nodeCount := len(roster.List)

	// Seed, compute permutation, get permuted list of node ID
	rand.Seed(seed)
	perm := rand.Perm(nodeCount)
	permutedIDs := getPermutedServerIDs(roster, perm)

	// Compute the shard groups: a shard group is the list of node ID assigned to a shard
	shardGroups := getShardGroups(shardCount, nodeCount, permutedIDs)

	// Finally, create the shard rosters from the shard groups
	shardRosters := make([]onet.Roster, shardCount)
	for i := 0; i < len(shardRosters); i++ {
		roster := onet.NewRoster(shardGroups[i])
		shardRosters[i] = *roster
	}

	return shardRosters
}

func getPermutedServerIDs(roster *onet.Roster, permutation []int) []*network.ServerIdentity {
	permutedIDs := make([]*network.ServerIdentity, 0)
	for _, v := range permutation {
		permutedIDs = append(permutedIDs, roster.List[v])
	}

	return permutedIDs
}

func getShardGroups(shardCount int, nodeCount int, permutedIDs []*network.ServerIdentity) [][]*network.ServerIdentity {
	// Compute the shard groups: a shard group is the list of node ID belonging to a shard
	// After this step, every shard group has the same size
	shardGroups := make([][]*network.ServerIdentity, shardCount)
	shardSize := nodeCount / shardCount
	for i := 0; i < shardCount; i++ {
		shardGroups[i] = permutedIDs[i*shardSize : (i+1)*shardSize]
	}

	// Assign the leftover nodes in a "fair" manner
	rest := permutedIDs[nodeCount-nodeCount%shardCount : nodeCount]
	j := 0
	for i := 0; i < len(rest); i++ {
		if j == len(shardGroups) {
			j = 0
		}
		shardGroups[j] = append(shardGroups[j], rest[i])
		j++
	}

	return shardGroups
}
