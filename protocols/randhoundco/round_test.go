package randhoundco

import (
	"sync"
	"testing"

	"github.com/alecthomas/assert"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/jvss"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/poly"
)

func TestMain(m *testing.M) {
	log.SetUseColors(true)
	log.MainTest(m, 3)
}

func TestJVSSCosiProto(t *testing.T) {
	// number of total nodes participating - except the client
	var nbNodes int = 10
	//  number of JVSS groups
	var nbGroups int = 2
	var msg = []byte("Hello World")
	// Generate the entities and groups
	local := sda.NewLocalTest()
	defer local.CloseAll()
	conodes := local.GenConodes(nbNodes)
	groups := groups(conodes, nbGroups)

	// launch all JVSS protocol for all groups and get the longterms
	secrets, jvssProtos := launchJVSS(groups, local)

	// Create client + the client-and-leaders tree
	client := local.NewConode(0)
	list := make([]*network.ServerIdentity, len(groups)+1)
	for i := range groups {
		// take the first entry in the list as the leader
		list[i+1] = groups[i][0].ServerIdentity
		idx := i
		groups[i][0].ProtocolRegister(ProtoName, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
			return NewRoundLeader(n, jvssProtos[idx])
		})
	}
	list[0] = client.ServerIdentity
	el := sda.NewRoster(list)
	tree := el.GenerateBinaryTree()
	log.Print("Tree is:")
	log.Print(tree.Dump())
	// compute the aggregation of the longterms
	aggLongterm := network.Suite.Point().Null()
	for _, s := range secrets {
		aggLongterm.Add(aggLongterm, s.Pub.SecretCommit())
	}

	// start the protocol
	tni := local.Overlays[client.ServerIdentity.ID].NewTreeNodeInstanceFromProtoName(tree, ProtoName)
	p, err := NewRoundClient(tni, msg, aggLongterm)
	log.ErrFatal(err)
	log.ErrFatal(local.Overlays[client.ServerIdentity.ID].RegisterProtocolInstance(p))
	roundCl := p.(*roundClient)

	var sigCh = make(chan []byte)
	roundCl.RegisterOnSignature(func(sig []byte) {
		sigCh <- sig
	})

	log.Print("Starting protoClient")
	go p.Start()

	// verify the signature
	sig := <-sigCh
	assert.Nil(t, VerifySignature(network.Suite, aggLongterm, msg, sig))
}

// Launch all JVSS groups and recolts the Longterm shares and the protocols of
// each leaders of the group.
func launchJVSS(groups [][]*sda.Conode, local *sda.LocalTest) ([]*poly.SharedSecret, []*jvss.JVSS) {
	// longterms of each groups
	longterms := make([]*poly.SharedSecret, len(groups))
	// protocols reference for each leader
	leaders := make([]*jvss.JVSS, len(groups))
	wg := &sync.WaitGroup{}
	for i, group := range groups {
		// create roster + tree for this JVSS group
		list := make([]*network.ServerIdentity, len(group))
		for j := range list {
			list[j] = group[j].ServerIdentity
		}
		el := sda.NewRoster(list)
		tree := el.GenerateBinaryTree()
		p, err := local.CreateProtocol("JVSSCoSi", tree)
		if err != nil {
			panic(err)
		}
		leader := p.(*jvss.JVSS)
		leaders[i] = leader
		wg.Add(1)
		// start this JVSS group and collect the longterm
		go func(idx int, leader *jvss.JVSS) {
			log.Print("Got leader", idx, "starting!")
			if err := leader.Start(); err != nil {
				panic(err)
			}
			lg := leader.Longterm()
			log.Print("Got leader", idx, "longterms")
			longterms[idx] = lg
			wg.Done()
		}(i, leader)
	}
	wg.Wait()

	return longterms, leaders
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
			shard = []*sda.Conode{}
		}
	}
	return groups
}
