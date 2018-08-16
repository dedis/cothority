package contracts

import (
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/require"
)

func TestValue_Spawn(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(2, true)
	cl := omniledger.NewClient()

	genesisMsg, err := omniledger.DefaultGenesisMsg(omniledger.CurrentVersion, roster,
		[]string{"spawn:value", "spawn:darc"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	_, err = cl.CreateGenesisBlock(genesisMsg)
	require.Nil(t, err)

	myvalue := []byte("1234")
	ctx := omniledger.ClientTransaction{
		Instructions: []omniledger.Instruction{{
			InstanceID: omniledger.NewInstanceID(gDarc.GetBaseID()),
			Nonce:      omniledger.Nonce{},
			Index:      0,
			Length:     1,
			Spawn: &omniledger.Spawn{
				ContractID: ContractValueID,
				Args: []omniledger.Argument{{
					Name:  "value",
					Value: myvalue,
				}},
			},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignBy(gDarc.GetBaseID(), signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)
	pr, err := cl.WaitProof(omniledger.NewInstanceID(ctx.Instructions[0].Hash()), genesisMsg.BlockInterval, myvalue)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match())
	values, err := pr.InclusionProof.RawValues()
	require.Nil(t, err)
	require.Equal(t, myvalue, values[0])

	local.WaitDone(genesisMsg.BlockInterval)
}
