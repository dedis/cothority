package lib

import (
	"bytes"
	"encoding/binary"

	"github.com/dedis/onet"
	"math/rand"
	"time"

	"github.com/dedis/onet/network"
)

// ChainConfig stores configuration information of one omniledger.
// It is stored inside the omniledger in question.
type ChainConfig struct {
	Roster       *onet.Roster
	ShardCount   int
	EpochSize    time.Duration
	Timestamp    time.Time
	ShardRosters []onet.Roster
}

// ChangeRoster changes oldRoster into newRoster, one change at the time.
// The algorithm starts by adding new nodes, then removing old ones.
// Changes are applied one at the time, that is the output roster will
// differ by one node from the input (except if input and output roster
// have the same nodes already). The function must be called multiple
// times to apply all the changes.
//
// The order in the roster is important since the first node is considered leader.
// Thus, once all new nodes have been added, the roster order is changed such that the new nodes
// are at the start of the roster.
//
// Example:
// 1st call: oldRoster = {A,B}, newRoster = {C,D}, returned roster = {A,B,C}
// 2nd call: oldRoster = {A,B,C}, newRoster = {C,D}, returned roster = {C,D,A,B} => Order has changed, C and D are now at the start of the roster
// 3rd call: oldRoster = {C,D,A,B, newRoster = {C,D}, returned roster = {C,D,B}
// 4th call: oldRoster = {B,C,D}, newRoster = {C,D}, returned roster = {C,D}
//
// Input:
//		- oldRoster - The current Roster we want to change
//		- newRoster - The target Roster
// Output:
//		- A Roster with one change applied
func ChangeRoster(oldRoster, newRoster onet.Roster) onet.Roster {
	intermList := append([]*network.ServerIdentity{}, oldRoster.List...)
	newList := newRoster.List

	oldMap := make(map[network.ServerIdentityID]bool)
	for _, o := range intermList {
		oldMap[o.ID] = true
	}

	newMap := make(map[network.ServerIdentityID]bool)
	for _, n := range newList {
		newMap[n.ID] = true
	}

	// Add new element of newRoster to oldRoster, one at the time
	for ind, n := range newList {
		if _, ok := oldMap[n.ID]; !ok {
			intermList = append(intermList, n)

			if ind == len(newList)-1 {
				// Once all new nodes have been added, change the roster order to put new nodes at the start of the list
				intermList = newList
				for _, n := range oldRoster.List {
					if _, ok := newMap[n.ID]; !ok {
						intermList = append(intermList, n)
					}
				}
			}

			return *onet.NewRoster(intermList)
		}
	}

	// Remove old element of oldRoster, one at the time
	for i, o := range intermList {
		if _, ok := newMap[o.ID]; !ok {
			intermList = append(intermList[:i], intermList[i+1:]...)
			return *onet.NewRoster(intermList)
		}
	}

	// If oldRoster and newRoster have the same nodes
	return newRoster
}

// EncodeDuration encodes a time.Duration into a byte array.
// Input:
//		- d - a time.Duration
// Output:
//		- The byte array encoding of d
func EncodeDuration(d time.Duration) []byte {
	durationInNs := int64(d * time.Nanosecond)
	tBuf := make([]byte, 8)
	binary.PutVarint(tBuf, durationInNs)

	return tBuf
}

// DecodeDuration decodes a byte array into a time.Duration.
// Input:
//		- dBuf - A byte array
// Output:
//		- The decoded time duration if succesful, nil-duration otherwise
//		- An error if any, nil otherwise
func DecodeDuration(dBuf []byte) (time.Duration, error) {
	decoded, err := binary.ReadVarint(bytes.NewBuffer(dBuf))
	if err != nil {
		return time.Duration(0), err
	}

	duration := time.Duration(int64(decoded)) * time.Nanosecond

	return duration, nil
}

// Sharding creates shard roster by partitioning an input roster.
// The partitioning is pseudo random.
// Each shard roster is a subset of the input roster.
// Input:
//		- roster - The roster to be partitioned
//		- shardCount - The number of shards
//		- seed - An initial seed for the pseudo random process
// Output:
//		- A Roster array containing the shard rosters
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
		shardRoster := onet.NewRoster(shardGroups[i])
		shardRosters[i] = *shardRoster
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
