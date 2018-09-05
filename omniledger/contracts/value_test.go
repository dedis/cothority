package contracts

import (
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	ol "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/require"
)

func TestValue_Spawn(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(2, true)

	genesisMsg, err := ol.DefaultGenesisMsg(ol.CurrentVersion, roster,
		[]string{"spawn:value", "spawn:darc"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	cl := ol.NewClient(ol.Config{Roster: *roster})
	_, err = cl.CreateGenesisBlock(genesisMsg)
	require.Nil(t, err)

	myvalue := []byte("1234")
	ctx := ol.ClientTransaction{
		Instructions: []ol.Instruction{{
			InstanceID: ol.NewInstanceID(gDarc.GetBaseID()),
			Nonce:      ol.Nonce{},
			Index:      0,
			Length:     1,
			Spawn: &ol.Spawn{
				ContractID: ContractValueID,
				Args: []ol.Argument{{
					Name:  "value",
					Value: myvalue,
				}},
			},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignBy(gDarc.GetBaseID(), signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)
	pr, err := cl.WaitProof(ol.NewInstanceID(ctx.Instructions[0].DeriveID("").Slice()), genesisMsg.BlockInterval, myvalue)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match())
	values, err := pr.InclusionProof.RawValues()
	require.Nil(t, err)
	require.Equal(t, myvalue, values[0])

	local.WaitDone(genesisMsg.BlockInterval)
}
