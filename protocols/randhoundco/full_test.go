package randhoundco

import (
	"sync"
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/require"
)

const FullProtoTest = FullProto + "Test"

func TestFull(t *testing.T) {
	//log.TestOutput(true, 3)
	// number of total nodes participating - except the client
	var nbNodes int = 10
	//  number of JVSS groups
	var nbGroups int = 2
	var protos []*fullProto
	var prMut sync.Mutex
	sda.GlobalProtocolRegister(FullProtoTest, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		p, err := NewNodeProtocol(n)
		prMut.Lock()
		protos = append(protos, p.(*fullProto))
		prMut.Unlock()
		return p, err
	})

	// Generate the entities and groups
	local := sda.NewLocalTest()
	defer local.CloseAll()
	conodes := local.GenConodes(nbNodes)
	coGroups, groupRequests, roster := groupsSetup(conodes, nbGroups)

	tree := roster.GenerateBinaryTree()
	require.True(t, tree.Root.ServerIdentity.Equal(coGroups[0][0].ServerIdentity))

	tni := local.Overlays[coGroups[0][0].ServerIdentity.ID].NewTreeNodeInstanceFromProtoName(tree, FullProtoTest)
	p, err := NewRootProtocol(tni, groupRequests)
	log.ErrFatal(err)
	err = local.Overlays[coGroups[0][0].ServerIdentity.ID].RegisterProtocolInstance(p)
	log.ErrFatal(err)

	// blocking
	p.Start()
	// get the groups
	groups := p.(*fullProto).Groups()
	groups.Dump()
	agg := p.(*fullProto).Suite().Point().Null()
	for _, g := range groups.Groups {
		agg.Add(agg, g.Longterm)
	}

	require.True(t, agg.Equal(groups.Aggregate))

	msg := []byte("Hello World")
	for i := 0; i < 2; i++ {
		sig, err := p.(*fullProto).NewRound(msg)
		log.ErrFatal(err)

		log.ErrFatal(VerifySignature(network.Suite, agg, msg, sig))
	}
	p.(*fullProto).Done()
	for _, pr := range protos {
		pr.Done()
	}

}
