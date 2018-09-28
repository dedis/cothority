package contracts

import (
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/require"
)

func TestValue_Spawn(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(2, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:value", "spawn:darc"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)

	myvalue := []byte("1234")
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
			Nonce:      byzcoin.Nonce{},
			Index:      0,
			Length:     1,
			Spawn: &byzcoin.Spawn{
				ContractID: ContractValueID,
				Args: []byzcoin.Argument{{
					Name:  "value",
					Value: myvalue,
				}},
			},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignBy(gDarc.GetBaseID(), signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)
	pr, err := cl.WaitProof(byzcoin.NewInstanceID(ctx.Instructions[0].DeriveID("").Slice()), genesisMsg.BlockInterval, myvalue)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match())
	values, err := pr.InclusionProof.RawValues()
	require.Nil(t, err)
	require.Equal(t, myvalue, values[0])

	local.WaitDone(genesisMsg.BlockInterval)
}
