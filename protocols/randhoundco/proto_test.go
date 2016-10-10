package proto

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestJVSSCosiProto(t *testing.T) {
	// number of total nodes participating - except the client
	var nbNodes int = 10
	//  number of JVSS groups
	var nbGroups int = 3
	// Generate the entities and groups

}

// Generate <groups> groups from the list of server identity.
// XXX To be moved to a general util package or something
func groups(sis []*network.ServerIdentity, nbGroup int) [][]*network.ServerIdentity {
	shard := []*network.ServerIdentity{}
	groups := [][]*network.ServerIdentity{}
	n := len(sis) / nbGroup
	for i := 0; i < len(sis); i++ {
		shard = append(shard, sis[i])
		if (i%n == n-1) || (i == len(sis)-1) {
			groups = append(groups, shard)
			shard := []*network.ServerIdentity{}
		}
	}
	return groups
}
