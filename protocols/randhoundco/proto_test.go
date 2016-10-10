package proto

import (
	"sync"
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/jvss"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/poly"
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
	local := sda.NewLocalTest()
	defer local.CloseAll()
	hosts, _, _ := local.GenHosts(nbNodes)
	groups = groups(hosts, nbGroups)

	// launch all JVSS protocol for all groups and get the longterms
	launchJVSS(groups, local)

	// Create client + the client-and-leaders tree
	client := local.GenLostsHosts(1)[0]
	list := make([]*network.ServerIdentity, len(groups)+1)
	for i := range groups {
		// take the first entry in the list as the leader
		list[i+1] = groups[i][0]
	}
	list[0] = client.ServerIdentity
	el := sda.NewRoster(list)
	tree := el.GenerateBinaryTree()

	// launch the protocol
	// XXX Needs to have to NewProtocol registration per Conode to give the
	// right JVSS

}

// Launch all JVSS groups and recolts the Longterm shares and the protocols of
// each leaders of the group.
func launchJVSS(groups [][]*sda.Conode, local *sda.Local) ([]*poly.SharedSecret, []*jvss.JVSS) {
	// longterms of each groups
	longterms := make([]*poly.SharedSecret, len(group))
	// protocols reference for each leader
	leaders := make([]*jvss.JVSS, len(group))
	wg := &sync.WaitGroup{}
	for i, group := range groups {
		// create roster + tree for this JVSS group
		list := make([]*network.ServerIdentity, len(group))
		for j := range list {
			list[j] = group[j].ServerIdentity
		}
		el := sda.NewRoster(list)
		tree := el.GenerateBinaryTree()
		leader, err := loca.CreateProtocol("JVSSCoSi", tree)
		if err != nil {
			panic(err)
		}

		leaders[i] = leader
		wg.Add(1)
		// start this JVSS group and collect the longterm
		go func(idx int, leader *jvss.JVSS) {
			if err := leader.Start(); err != nil {
				panic(err)
			}
			lg := leader.Longterm()
			longterms[idx] = lg
			wg.Done()
		}(i, leader)
	}
	wg.Wait()

	return leaders, longterms
}

// Generate <groups> groups from the list of server identity.
// XXX To be moved to a general util package or something
func groups(sis []*sda.Conode, nbGroup int) [][]*sda.Conode {
	shard := []*sda.Conode{}
	groups := [][]*sda.Conode{}
	n := len(sis) / nbGroup
	for i := 0; i < len(sis); i++ {
		shard = append(shard, sis[i])
		if (i%n == n-1) || (i == len(sis)-1) {
			groups = append(groups, shard)
			shard := []*sda.Conode{}
		}
	}
	return groups
}
