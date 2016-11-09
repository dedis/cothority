package randhoundco

import (
	"crypto/rand"
	"sync"
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/jvss"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	// number of total nodes participating - except the client
	var nbNodes int = 15
	//  number of JVSS groups
	var nbGroups int = 3
	// Generate the entities and groups
	local := sda.NewLocalTest()
	defer local.CloseAll()
	conodes := local.GenConodes(nbNodes)
	coGroups, groupRequests, roster := groupsSetup(conodes, nbGroups)
	// register the protocol instantiation to get all the jvss instances
	jvssProtos := make([]*jvss.JVSS, 0, len(conodes))
	var jvMut sync.Mutex
	for _, c := range coGroups[1:] {
		_, err := c[0].ProtocolRegister(SetupProto, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
			setup, err := NewSetupNode(n)
			setup.RegisterOnJVSS(func(jv *jvss.JVSS) {
				jvMut.Lock()
				defer jvMut.Unlock()
				jvssProtos = append(jvssProtos, jv)
			})
			return setup, err
		})
		log.ErrFatal(err)
	}

	// channel to indicate when the setup is finished
	var grCh = make(chan *Groups)
	// register for the root
	_, err := coGroups[0][0].ProtocolRegister(SetupProto, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		setup, err := NewSetupRoot(n, groupRequests)
		setup.RegisterOnJVSS(func(jv *jvss.JVSS) {
			jvMut.Lock()
			defer jvMut.Unlock()
			jvssProtos = append(jvssProtos, jv)
		})
		setup.RegisterOnSetupDone(func(g *Groups) {
			grCh <- g
		})
		return setup, err
	})
	log.ErrFatal(err)

	tree := roster.GenerateBinaryTree()
	p, err := local.CreateProtocol(SetupProto, tree)
	log.ErrFatal(err)

	// launch it
	go p.Start()

	groupsCreated := <-grCh
	require.True(t, len(jvssProtos) > 0)
	// compute the aggregated and check if it's correct
	aggLongterm := network.Suite.Point().Null()
	for _, jv := range jvssProtos {
		aggLongterm.Add(aggLongterm, jv.Longterm().Pub.SecretCommit())
	}

	if !aggLongterm.Equal(groupsCreated.Aggregate) {
		t.Error("longterms are not equals?")
	}
}

func groupsSetup(list []*sda.Conode, nbGroup int) ([][]*sda.Conode, GroupRequests, *sda.Roster) {
	var shard []*network.ServerIdentity
	var coShard []*sda.Conode
	var groups []GroupRequest
	var coGroups [][]*sda.Conode
	n := len(list) / nbGroup
	// add the client identity to the roster
	leaders := []*network.ServerIdentity{}
	var leadersIdx []int32
	for i := 0; i < len(list); i++ {
		shard = append(shard, list[i].ServerIdentity)
		coShard = append(coShard, list[i])
		if len(shard) == 1 {
			leaders = append(leaders, list[i].ServerIdentity)
			leadersIdx = append(leadersIdx, int32(len(groups)))
		}
		if (i%n == n-1) && len(groups) < nbGroup-1 {
			groups = append(groups, GroupRequest{shard})
			coGroups = append(coGroups, coShard)
			shard = []*network.ServerIdentity{}
			coShard = []*sda.Conode{}
		}
	}
	groups = append(groups, GroupRequest{shard})
	coGroups = append(coGroups, coShard)
	// generate the random identifier
	// XXX This step will also be replaced by the randhound protocol's output
	// once merged.
	var id [16]byte
	n, err := rand.Read(id[:])
	if n != 16 || err != nil {
		panic("the whole system is compromised, leave the ship")
	}
	g := GroupRequests{id[:], groups, leadersIdx}
	roster := sda.NewRoster(leaders)
	return coGroups, g, roster
}
