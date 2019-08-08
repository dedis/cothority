package contracts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
)

func TestValue_Spawn(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:value"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)

	myvalue := []byte("1234")
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: ContractValueID,
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			}},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)
	pr, err := cl.WaitProof(byzcoin.NewInstanceID(ctx.Instructions[0].DeriveID("").Slice()), 2*genesisMsg.BlockInterval, myvalue)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(ctx.Instructions[0].DeriveID("").Slice()))
	v0, _, _, err := pr.Get(ctx.Instructions[0].DeriveID("").Slice())
	require.Nil(t, err)
	require.Equal(t, myvalue, v0)

	local.WaitDone(genesisMsg.BlockInterval)
}

// This test uses the same code as the Spawn one but then performs an update
// on the value contract.
func TestValue_Invoke(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:value", "invoke:value.update"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)

	myvalue := []byte("1234")
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: ContractValueID,
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			}},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	pr, err := cl.WaitProof(byzcoin.NewInstanceID(myID.Slice()), 2*genesisMsg.BlockInterval, myvalue)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(myID.Slice()))

	v0, _, _, err := pr.Get(myID.Slice())
	require.Nil(t, err)
	require.Equal(t, myvalue, v0)

	local.WaitDone(genesisMsg.BlockInterval)

	//
	// Invoke part
	//
	myvalue = []byte("5678")
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: ContractValueID,
			Command:    "update",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			}},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)

	pr, err = cl.WaitProof(byzcoin.NewInstanceID(myID.Slice()), 2*genesisMsg.BlockInterval, myvalue)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(myID.Slice()))

	v0, _, _, err = pr.Get(myID.Slice())
	require.Nil(t, err)
	require.Equal(t, myvalue, v0)

	local.WaitDone(genesisMsg.BlockInterval)
}
