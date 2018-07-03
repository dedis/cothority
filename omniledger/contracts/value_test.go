package contracts

import (
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/require"
)

func TestValue_Spawn(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	servers, roster, _ := local.GenTree(2, true)
	cl := service.NewClient()

	genesisMsg, err := service.DefaultGenesisMsg(service.CurrentVersion, roster,
		[]string{"spawn:value", "spawn:darc"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	_, err = cl.CreateGenesisBlock(genesisMsg)
	require.Nil(t, err)

	myvalue := []byte("1234")
	ctx := service.ClientTransaction{
		Instructions: []service.Instruction{{
			InstanceID: service.InstanceID{
				DarcID: gDarc.GetBaseID(),
				SubID:  service.SubID{},
			},
			Nonce:  service.Nonce{},
			Index:  0,
			Length: 1,
			Spawn: &service.Spawn{
				ContractID: ContractValueID,
				Args: []service.Argument{{
					Name:  "value",
					Value: myvalue,
				}},
			},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignBy(signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)
	instID := service.InstanceID{
		DarcID: ctx.Instructions[0].InstanceID.DarcID,
		SubID:  service.NewSubID(ctx.Instructions[0].Hash()),
	}
	pr, err := cl.WaitProof(instID, genesisMsg.BlockInterval, myvalue)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match())
	values, err := pr.InclusionProof.RawValues()
	require.Nil(t, err)
	require.Equal(t, myvalue, values[0])

	services := local.GetServices(servers, service.OmniledgerID)
	for _, s := range services {
		close(s.(*service.Service).CloseQueues)
	}
	local.WaitDone(genesisMsg.BlockInterval)
}
