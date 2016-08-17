package randhound

import (
	"fmt"

	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

// Shard produces a pseudorandom sharding of the network entity list
// based on a seed and a number of requested shards.
func (rh *RandHound) Shard(seed []byte, shards uint32) ([][]*sda.TreeNode, [][]abstract.Point, error) {

	nodes := rh.Transcript.Session.Nodes

	if shards == 0 || nodes < shards {
		return nil, nil, fmt.Errorf("Number of requested shards not supported")
	}

	// Compute a random permutation of [1,n-1]
	prng := rh.Suite().Cipher(seed)
	m := make([]uint32, nodes-1)
	for i := range m {
		j := int(random.Uint64(prng) % uint64(i+1))
		m[i] = m[j]
		m[j] = uint32(i) + 1
	}

	// Create sharding of the current Roster according to the above permutation
	el := rh.List()
	n := int(nodes / shards)
	sharding := [][]*sda.TreeNode{}
	shard := []*sda.TreeNode{}
	keys := [][]abstract.Point{}
	k := []abstract.Point{}
	for i, j := range m {
		shard = append(shard, el[j])
		k = append(k, el[j].ServerIdentity.Public)
		if (i%n == n-1) || (i == len(m)-1) {
			sharding = append(sharding, shard)
			shard = make([]*sda.TreeNode, 0)
			keys = append(keys, k)
			k = make([]abstract.Point, 0)
		}
	}
	return sharding, keys, nil
}
