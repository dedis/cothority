package contracts

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/protobuf"

	"go.dedis.ch/cothority/v3/darc"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
)

func TestDeferred_Spawn(t *testing.T) {
	// In this test I am just trying to see if a spawn successfully stores
	// the given argument and if I am able to retrieve them after. It was
	// interesting to play with the encode/decode protobuf.
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:deferred"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)

	proposedTransaction := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{
			byzcoin.Instruction{
				InstanceID: byzcoin.InstanceID{0x10, 0x11, 0x12},
				Spawn: &byzcoin.Spawn{
					ContractID: "value",
					Args: byzcoin.Arguments{
						byzcoin.Argument{
							Name:  "test",
							Value: []byte("1234"),
						},
					},
				},
			},
		},
	}
	expireSec := []byte("6000")
	expireSecInt, _ := strconv.ParseUint(string(expireSec), 10, 64)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: ContractDeferredID,
				Args: []byzcoin.Argument{
					{
						Name:  "proposedTransaction",
						Value: proposedTransactionBuf,
					},
					{
						Name:  "expireSec",
						Value: expireSec,
					},
				},
			},
			SignerCounter: []uint64{1},
		}},
	}
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)

	pr, err := cl.WaitProof(byzcoin.NewInstanceID(ctx.Instructions[0].DeriveID("").Slice()), 2*genesisMsg.BlockInterval, nil)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(ctx.Instructions[0].DeriveID("").Slice()))

	dataBuf, _, _, err := pr.Get(ctx.Instructions[0].DeriveID("").Slice())
	require.Nil(t, err)
	result := DeferredData{}
	err = protobuf.Decode(dataBuf, &result)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, result.ExpireSec, expireSecInt)
	require.NotEmpty(t, result.Timestamp)

	local.WaitDone(genesisMsg.BlockInterval)
}

func TestDeferred_Scenario(t *testing.T) {
	// Since every method relies on the execution of a previous ones, I am not
	// unit test but rather creating a scenario:
	//
	// 1. Spawn a new contract
	// 2. Invoke two "addProff"
	// 3. Invoke an "execRoot"

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:value", "spawn:deferred", "invoke:deferred.addProof",
			"invoke:deferred.execProposedTx"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)

	rootInstructionValue := []byte("aef123456789fab")

	proposedTransaction := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{
			byzcoin.Instruction{
				InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
				Spawn: &byzcoin.Spawn{
					ContractID: "value",
					Args: byzcoin.Arguments{
						byzcoin.Argument{
							Name:  "value",
							Value: rootInstructionValue,
						},
					},
				},
			},
		},
	}

	expireSec := []byte("6000")
	expireSecInt, _ := strconv.ParseUint(string(expireSec), 10, 64)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: ContractDeferredID,
				Args: []byzcoin.Argument{
					{
						Name:  "proposedTransaction",
						Value: proposedTransactionBuf,
					},
					{
						Name:  "expireSec",
						Value: expireSec,
					},
				},
			},
			SignerCounter: []uint64{1},
		}},
	}
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	pr, err := cl.WaitProof(byzcoin.NewInstanceID(myID.Slice()), 2*genesisMsg.BlockInterval, nil)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(myID.Slice()))

	dataBuf, _, _, err := pr.Get(myID.Slice())
	require.Nil(t, err)
	result := DeferredData{}
	err = protobuf.Decode(dataBuf, &result)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	// require.Equal(t, result.ProposedTransaction.Instructions[0].Spawn.Args.Search("value"), 1)
	require.Equal(t, result.ExpireSec, expireSecInt)
	require.NotEmpty(t, result.Timestamp)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	local.WaitDone(genesisMsg.BlockInterval)

	rootHash := result.Hash

	// ------------------------------------------------------------------------
	// 2.1 Invoke a first "addProof"
	// ------------------------------------------------------------------------

	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err := signer.Sign(rootHash)
	require.Nil(t, err)
	// signature[1] = 0xf

	ctx = byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: myID,
			Invoke: &byzcoin.Invoke{
				ContractID: ContractDeferredID,
				Command:    "addProof",
				Args: []byzcoin.Argument{
					{
						Name:  "identity",
						Value: identityBuf,
					},
					{
						Name:  "signature",
						Value: signature,
					},
				},
			},
			SignerCounter: []uint64{2},
		}},
	}
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)

	proposedTransaction.Instructions[0].SignerIdentities = append(proposedTransaction.Instructions[0].SignerIdentities, identity)
	proposedTransaction.Instructions[0].Signatures = append(proposedTransaction.Instructions[0].Signatures, signature)
	result.ProposedTransaction = proposedTransaction
	resultBuf, err := protobuf.Encode(&result)
	require.Nil(t, err)

	pr, err = cl.WaitProof(byzcoin.NewInstanceID(myID.Slice()), 2*genesisMsg.BlockInterval, resultBuf)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(myID.Slice()))

	dataBuf, _, _, err = pr.Get(myID.Slice())
	require.Nil(t, err)

	result = DeferredData{}
	err = protobuf.Decode(dataBuf, &result)
	require.Nil(t, err)

	// We can not do this test because the identities have to be compared using
	// the Equal() method.
	//require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireSec, expireSecInt)
	require.NotEmpty(t, result.Timestamp)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].SignerIdentities), 1)
	// Surprisingly this test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[0].SignerIdentities[0]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].Signatures), 1)
	require.Equal(t, result.ProposedTransaction.Instructions[0].Signatures[0], signature)

	require.NotEmpty(t, result.Hash)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 2.1 Invoke a second "addProof"
	// ------------------------------------------------------------------------
	//
	// Lets try to add another signer. Here I am still using the previous signer
	// to send the transaction because he has the right to. I am just trying to
	// see if adding another new identity and signature will result in having
	// two identities and two signatures.
	//

	signer2 := darc.NewSignerEd25519(nil, nil)
	identity = signer2.Identity()
	identityBuf, err = protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err = signer2.Sign(rootHash)
	require.Nil(t, err)
	signature[1] = 0xf // Simulates a wrong signature

	ctx = byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: myID,
			Invoke: &byzcoin.Invoke{
				ContractID: ContractDeferredID,
				Command:    "addProof",
				Args: []byzcoin.Argument{
					{
						Name:  "identity",
						Value: identityBuf,
					},
					{
						Name:  "signature",
						Value: signature,
					},
				},
			},
			SignerCounter: []uint64{3},
		}},
	}
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)

	proposedTransaction.Instructions[0].SignerIdentities = append(proposedTransaction.Instructions[0].SignerIdentities, identity)
	proposedTransaction.Instructions[0].Signatures = append(proposedTransaction.Instructions[0].Signatures, signature)
	result.ProposedTransaction = proposedTransaction
	resultBuf, err = protobuf.Encode(&result)
	require.Nil(t, err)

	pr, err = cl.WaitProof(byzcoin.NewInstanceID(myID.Slice()), 2*genesisMsg.BlockInterval, resultBuf)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(myID.Slice()))

	dataBuf, _, _, err = pr.Get(myID.Slice())
	require.Nil(t, err)

	result = DeferredData{}
	err = protobuf.Decode(dataBuf, &result)
	require.Nil(t, err)

	// We can not do this test because the identities have to be compared using
	// the Equal() method.
	//require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireSec, expireSecInt)
	require.NotEmpty(t, result.Timestamp)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].SignerIdentities), 2)
	// Surprisingly this test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[0].SignerIdentities[1]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].Signatures), 2)
	require.Equal(t, result.ProposedTransaction.Instructions[0].Signatures[1], signature)

	require.NotEmpty(t, result.Hash)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 3. Invoke an "execRoot" command
	// ------------------------------------------------------------------------

	ctx = byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: myID,
			Invoke: &byzcoin.Invoke{
				ContractID: ContractDeferredID,
				Command:    "execProposedTx",
			},
			SignerCounter: []uint64{4},
		}},
	}
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransaction(ctx)
	require.Nil(t, err)

	// Need to sleep because we can't predict the output (hence the 'nil')
	time.Sleep(2 * genesisMsg.BlockInterval)
	pr, err = cl.WaitProof(byzcoin.NewInstanceID(myID.Slice()), 2*genesisMsg.BlockInterval, nil)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(myID.Slice()))
	dataBuf, _, _, err = pr.Get(myID.Slice())
	require.Nil(t, err)

	local.WaitDone(genesisMsg.BlockInterval)

	time.Sleep(2 * genesisMsg.BlockInterval)
	pr, err = cl.WaitProof(byzcoin.NewInstanceID(dataBuf), 2*genesisMsg.BlockInterval, nil)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(dataBuf))

	valueRes, _, _, err := pr.Get(dataBuf)
	require.Nil(t, err)

	// Such a miracle to retrieve this value that was set at the begining
	require.Equal(t, valueRes, rootInstructionValue)

	local.WaitDone(genesisMsg.BlockInterval)
}
