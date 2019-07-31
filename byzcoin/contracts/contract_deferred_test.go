package contracts

import (
	"encoding/binary"
	"testing"
	"time"

	"go.dedis.ch/onet/v3/network"

	"github.com/stretchr/testify/require"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/protobuf"

	"go.dedis.ch/cothority/v3/darc"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
)

// Note: Those tests relie on the Value contract, hence it is not possible to
//       include this file in the byzcoin package.

func TestDeferred_ScenarioSingleInstruction(t *testing.T) {
	// Since every method relies on the execution of a previous ones, I am not
	// unit test but rather creating a scenario:
	//
	// 1. Spawn a new contract
	// 2. Invoke two "addProff"
	// 3. Invoke an "execRoot"

	// ------------------------------------------------------------------------
	// 0. Set up
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
	require.NoError(t, err)
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	rootInstructionValue := []byte("aef123456789fab")

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
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
	})
	require.NoError(t, err)

	expireBlockIndexInt := uint64(6000)
	expireBlockIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(expireBlockIndexBuf, expireBlockIndexInt)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
				{
					Name:  "expireBlockIndex",
					Value: expireBlockIndexBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	local.WaitDone(genesisMsg.BlockInterval)

	rootHash := result.InstructionHashes

	// ------------------------------------------------------------------------
	// 2.1 Invoke a first "addProof"
	// ------------------------------------------------------------------------

	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err := signer.Sign(rootHash[0]) // == index
	require.Nil(t, err)
	// signature[1] = 0xf

	index := uint32(0)
	indexBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	proposedTransaction.Instructions[0].SignerIdentities = append(proposedTransaction.Instructions[0].SignerIdentities, identity)
	proposedTransaction.Instructions[0].Signatures = append(proposedTransaction.Instructions[0].Signatures, signature)
	result.ProposedTransaction = proposedTransaction

	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].SignerIdentities), 1)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[0].SignerIdentities[0]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].Signatures), 1)
	require.Equal(t, result.ProposedTransaction.Instructions[0].Signatures[0], signature)
	// Default MaxNumExecution should be 1
	require.Equal(t, result.MaxNumExecution, uint64(1))

	require.NotEmpty(t, result.InstructionHashes)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 2.2 Invoke a second "addProof"
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

	signature, err = signer2.Sign(rootHash[0]) // == index
	require.Nil(t, err)
	// signature[1] = 0xf // Simulates a wrong signature

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	proposedTransaction.Instructions[0].SignerIdentities = append(proposedTransaction.Instructions[0].SignerIdentities, identity)
	proposedTransaction.Instructions[0].Signatures = append(proposedTransaction.Instructions[0].Signatures, signature)
	result.ProposedTransaction = proposedTransaction

	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].SignerIdentities), 2)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[0].SignerIdentities[1]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].Signatures), 2)
	require.Equal(t, result.ProposedTransaction.Instructions[0].Signatures[1], signature)

	require.NotEmpty(t, result.InstructionHashes)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 3. Invoke an "execRoot" command
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
			Command:    "execProposedTx",
		},
		SignerCounter: []uint64{4},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)
	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	result, err = cl.GetDeferredData(myID)
	require.Equal(t, 1, len(result.ExecResult))

	time.Sleep(2 * genesisMsg.BlockInterval)
	pr, err := cl.WaitProof(byzcoin.NewInstanceID(result.ExecResult[0]), 2*genesisMsg.BlockInterval, nil)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(result.ExecResult[0]))

	valueRes, _, _, err := pr.Get(result.ExecResult[0])
	require.Nil(t, err)

	// Such a miracle to retrieve this value that was set at the begining
	require.Equal(t, valueRes, rootInstructionValue)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// ------------------------------------------------------------------------
	// 4. Invoke an "execRoot" command a second time. Since MaxNumExecution should
	//    be at 0, we expect it to fail.
	//    NOTE: We are trying to spawn two times a contract with the sane id,
	//          which is not likely to create two instances. Here we are only
	//          testing if the check of the MaxNumExecution works.
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
			Command:    "execProposedTx",
		},
		SignerCounter: []uint64{5},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	local.WaitDone(genesisMsg.BlockInterval)
}

func TestDeferred_ScenarioMultiInstructions_(t *testing.T) {
	// Since every method relies on the execution of a previous ones, I am not
	// unit test but rather creating a scenario:
	//
	// 1. Spawn a new contract with two instruction
	// 2. Invoke two "addProff"
	// 3. Invoke an "execRoot"

	// ------------------------------------------------------------------------
	// 0. Set up
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
	require.NoError(t, err)
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	rootInstructionValue1 := []byte("aef123456789fab")
	rootInstructionValue2 := []byte("1234aef")

	// We spawn two value contracts
	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: "value",
			Args: byzcoin.Arguments{
				byzcoin.Argument{
					Name:  "value",
					Value: rootInstructionValue1,
				},
			},
		},
	}, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: "value",
			Args: byzcoin.Arguments{
				byzcoin.Argument{
					Name:  "value",
					Value: rootInstructionValue2,
				},
			},
		},
	})
	require.NoError(t, err)

	expireBlockIndexInt := uint64(6000)
	expireBlockIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(expireBlockIndexBuf, expireBlockIndexInt)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
				{
					Name:  "expireBlockIndex",
					Value: expireBlockIndexBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 2)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	local.WaitDone(genesisMsg.BlockInterval)

	rootHash := result.InstructionHashes

	// ------------------------------------------------------------------------
	// 2.1 Invoke a first "addProof" on the first instruction
	// ------------------------------------------------------------------------

	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err := signer.Sign(rootHash[0]) // == index
	require.Nil(t, err)
	// signature[1] = 0xf

	index := uint32(0)
	indexBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	proposedTransaction.Instructions[0].SignerIdentities = append(proposedTransaction.Instructions[0].SignerIdentities, identity)
	proposedTransaction.Instructions[0].Signatures = append(proposedTransaction.Instructions[0].Signatures, signature)
	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 2)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].SignerIdentities), 1)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[0].SignerIdentities[0]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].Signatures), 1)
	require.Equal(t, result.ProposedTransaction.Instructions[0].Signatures[0], signature)
	// Default MaxNumExecution should be 1
	require.Equal(t, result.MaxNumExecution, uint64(1))

	require.NotEmpty(t, result.InstructionHashes)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 2.2 Invoke a second "addProof" on the second instruction
	// ------------------------------------------------------------------------

	signature, err = signer.Sign(rootHash[1]) // == index
	require.Nil(t, err)

	index = uint32(1)
	indexBuf = make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	proposedTransaction.Instructions[1].SignerIdentities = append(proposedTransaction.Instructions[1].SignerIdentities, identity)
	proposedTransaction.Instructions[1].Signatures = append(proposedTransaction.Instructions[1].Signatures, signature)

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 2)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[1].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[1].SignerIdentities), 1)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[1].SignerIdentities[0]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[1].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[1].Signatures), 1)
	require.Equal(t, result.ProposedTransaction.Instructions[1].Signatures[0], signature)

	require.NotEmpty(t, result.InstructionHashes)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 3. Invoke an "execRoot" command
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
			Command:    "execProposedTx",
		},
		SignerCounter: []uint64{4},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	result, err = cl.GetDeferredData(myID)

	time.Sleep(2 * genesisMsg.BlockInterval)
	pr, err := cl.WaitProof(byzcoin.NewInstanceID(result.ExecResult[0]), 2*genesisMsg.BlockInterval, nil)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(result.ExecResult[0]))

	valueRes, _, _, err := pr.Get(result.ExecResult[0])
	require.Nil(t, err)

	// Such a miracle to retrieve this value that was set at the begining
	require.Equal(t, valueRes, rootInstructionValue1)

	time.Sleep(2 * genesisMsg.BlockInterval)
	pr, err = cl.WaitProof(byzcoin.NewInstanceID(result.ExecResult[1]), 2*genesisMsg.BlockInterval, nil)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(result.ExecResult[1]))

	valueRes, _, _, err = pr.Get(result.ExecResult[1])
	require.Nil(t, err)

	// Such a miracle to retrieve this value that was set at the begining
	require.Equal(t, valueRes, rootInstructionValue2)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))
}

func TestDeferred_ScenarioMultiInstructionsDifferentSigners(t *testing.T) {
	// I commit two instructions that are siged by two different signers. The
	// second signer has no right to sign the instruction, so we expect the transaction to fail.

	// ------------------------------------------------------------------------
	// 0. Set up
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
	require.NoError(t, err)
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	rootInstructionValue1 := []byte("aef123456789fab")
	rootInstructionValue2 := []byte("1234aef")

	// We spawn two value contracts
	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: "value",
			Args: byzcoin.Arguments{
				byzcoin.Argument{
					Name:  "value",
					Value: rootInstructionValue1,
				},
			},
		},
	}, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: "value",
			Args: byzcoin.Arguments{
				byzcoin.Argument{
					Name:  "value",
					Value: rootInstructionValue2,
				},
			},
		},
	})
	require.NoError(t, err)

	expireBlockIndexInt := uint64(6000)
	expireBlockIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(expireBlockIndexBuf, expireBlockIndexInt)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
				{
					Name:  "expireBlockIndex",
					Value: expireBlockIndexBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 2)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	rootHash := result.InstructionHashes

	// ------------------------------------------------------------------------
	// 2.1 Invoke a first "addProof" on the first instruction
	// ------------------------------------------------------------------------

	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err := signer.Sign(rootHash[0]) // == index
	require.Nil(t, err)
	// signature[1] = 0xf

	index := uint32(0)
	indexBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	proposedTransaction.Instructions[0].SignerIdentities = append(proposedTransaction.Instructions[0].SignerIdentities, identity)
	proposedTransaction.Instructions[0].Signatures = append(proposedTransaction.Instructions[0].Signatures, signature)

	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 2)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].SignerIdentities), 1)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[0].SignerIdentities[0]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].Signatures), 1)
	require.Equal(t, result.ProposedTransaction.Instructions[0].Signatures[0], signature)
	// Default MaxNumExecution should be 1
	require.Equal(t, result.MaxNumExecution, uint64(1))

	require.NotEmpty(t, result.InstructionHashes)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// ------------------------------------------------------------------------
	// 2.2 Invoke a second "addProof" on the second instruction, but with a
	//     different signer
	// ------------------------------------------------------------------------

	signer2 := darc.NewSignerEd25519(nil, nil)

	identity = signer2.Identity()
	identityBuf, err = protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err = signer2.Sign(rootHash[1]) // == index
	require.Nil(t, err)

	index = uint32(1)
	indexBuf = make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	proposedTransaction.Instructions[1].SignerIdentities = append(proposedTransaction.Instructions[1].SignerIdentities, identity)
	proposedTransaction.Instructions[1].Signatures = append(proposedTransaction.Instructions[1].Signatures, signature)

	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 2)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[1].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[1].SignerIdentities), 1)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[1].SignerIdentities[0]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[1].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[1].Signatures), 1)
	require.Equal(t, result.ProposedTransaction.Instructions[1].Signatures[0], signature)

	require.NotEmpty(t, result.InstructionHashes)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// ------------------------------------------------------------------------
	// 3. Invoke an "execRoot" command. This one will fail since one of the
	//    instruction is signed by an unauthorized signer.
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
			Command:    "execProposedTx",
		},
		SignerCounter: []uint64{4},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	myID = ctx.Instructions[0].DeriveID("")

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))
}

func TestDeferred_WrongSignature(t *testing.T) {
	// If a client performs an "addProof" with a wrong signature, then it should
	// produce an error and reject the transaction

	// ------------------------------------------------------------------------
	// 0. Set up
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
	require.NoError(t, err)
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	rootInstructionValue := []byte("aef123456789fab")

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
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
	})
	require.NoError(t, err)

	expireBlockIndexInt := uint64(6000)
	expireBlockIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(expireBlockIndexBuf, expireBlockIndexInt)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
				{
					Name:  "expireBlockIndex",
					Value: expireBlockIndexBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	local.WaitDone(genesisMsg.BlockInterval)

	rootHash := result.InstructionHashes

	// ------------------------------------------------------------------------
	// 2 Invoke an "addProof" with a wrong signature
	// ------------------------------------------------------------------------

	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err := signer.Sign(rootHash[0]) // == index
	require.Nil(t, err)
	signature[1] = 0xf

	index := uint32(0)
	indexBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	local.WaitDone(genesisMsg.BlockInterval)
}

func TestDeferred_DuplicateIdentity(t *testing.T) {
	// We do not store duplicates of identities. If someone tries to add an
	// identity that is already added, it returns an error.

	// ------------------------------------------------------------------------
	// 0. Set up
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
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	rootInstructionValue := []byte("aef123456789fab")

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
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
	})
	require.NoError(t, err)

	expireBlockIndexInt := uint64(6000)
	expireBlockIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(expireBlockIndexBuf, expireBlockIndexInt)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
				{
					Name:  "expireBlockIndex",
					Value: expireBlockIndexBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	local.WaitDone(genesisMsg.BlockInterval)

	rootHash := result.InstructionHashes

	// ------------------------------------------------------------------------
	// 2 Invoke an "addProof"
	// ------------------------------------------------------------------------

	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err := signer.Sign(rootHash[0]) // == index
	require.Nil(t, err)

	index := uint32(0)
	indexBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	_, err = cl.WaitProof(byzcoin.NewInstanceID(myID.Slice()), 2*genesisMsg.BlockInterval, nil)
	require.Nil(t, err)

	// ------------------------------------------------------------------------
	// 3 Invoke a second time the same "addProof"
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	local.WaitDone(genesisMsg.BlockInterval)
}

func TestDeferred_ExpireBlockIndex(t *testing.T) {
	// We set an "expireBlockIndex" to 0, which should prevent any invoke.

	// ------------------------------------------------------------------------
	// 0. Set up
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
	require.NoError(t, err)
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	rootInstructionValue := []byte("aef123456789fab")

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
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
	})
	require.NoError(t, err)

	expireBlockIndexInt := uint64(6000)
	expireBlockIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(expireBlockIndexBuf, expireBlockIndexInt)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
				{
					Name:  "expireBlockIndex",
					Value: expireBlockIndexBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	local.WaitDone(genesisMsg.BlockInterval)

	rootHash := result.InstructionHashes

	// ------------------------------------------------------------------------
	// 2 Invoke an "addProof"
	// ------------------------------------------------------------------------

	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err := signer.Sign(rootHash[0]) // == index
	require.Nil(t, err)

	index := uint32(0)
	indexBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	_, err = cl.WaitProof(byzcoin.NewInstanceID(ctx.Instructions[0].DeriveID("").Slice()), 2*genesisMsg.BlockInterval, nil)
	require.Error(t, err)

	local.WaitDone(genesisMsg.BlockInterval)
}

func TestDeferred_ExecWithNoProof(t *testing.T) {
	// We will sign the proposed transaction with no proof on it. We expect it
	// to fail

	// ------------------------------------------------------------------------
	// 0. Set up
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
	require.NoError(t, err)
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	rootInstructionValue := []byte("aef123456789fab")

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
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
	})
	require.NoError(t, err)

	expireBlockIndexInt := uint64(6000)
	expireBlockIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(expireBlockIndexBuf, expireBlockIndexInt)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
				{
					Name:  "expireBlockIndex",
					Value: expireBlockIndexBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 2. Invoke an "execProposedTx" command
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
			Command:    "execProposedTx",
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	local.WaitDone(genesisMsg.BlockInterval)
}

func TestDeferred_InstructionsDependent(t *testing.T) {
	// Here we run a deferred transaction and check if instructions can be
	// sequentially dependent. We simply test this by invoking a first
	// delete instruction on a value contract, then we try to read the deleted
	// contract. If we can't, we know instructions are sequentially dependent.
	//
	// 0.1.  Setup
	// 0.2.  Spawn a value contract
	// 1.    Spawn the deferred contract with two instructions
	// 2.    Invoke a first "addProof" to sign the proposed transaction
	// 3.    Invoke a second "addProof" to sign the proposed transaction
	// 4.    Invoke an "execProposedTx"

	// ------------------------------------------------------------------------
	// 0.1. Set up
	// ------------------------------------------------------------------------
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:value", "delete:value", "spawn:deferred", "invoke:deferred.addProof",
			"invoke:deferred.execProposedTx", "invoke:value.update"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.NoError(t, err)
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 0.2. Spawn a value contract
	// ------------------------------------------------------------------------

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

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	valueID := ctx.Instructions[0].DeriveID("")
	pr, err := cl.WaitProof(byzcoin.NewInstanceID(valueID.Slice()), 2*genesisMsg.BlockInterval, myvalue)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(valueID.Slice()))

	v0, _, _, err := pr.Get(valueID.Slice())
	require.Nil(t, err)
	require.Equal(t, myvalue, v0)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 1. Spawn our deferred contract. We provide the previous ID.
	// ------------------------------------------------------------------------
	rootInstructionValue := []byte("aef123456789fab")

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: valueID,
		Delete: &byzcoin.Delete{
			ContractID: "value",
		},
	}, byzcoin.Instruction{
		InstanceID: valueID,
		Invoke: &byzcoin.Invoke{
			ContractID: "value",
			Command:    "update",
			Args: byzcoin.Arguments{
				byzcoin.Argument{
					Name:  "value",
					Value: rootInstructionValue,
				},
			},
		},
	})
	require.NoError(t, err)

	expireBlockIndexInt := uint64(6000)
	expireBlockIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(expireBlockIndexBuf, expireBlockIndexInt)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
				{
					Name:  "expireBlockIndex",
					Value: expireBlockIndexBuf,
				},
			},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.NoError(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 2)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	local.WaitDone(genesisMsg.BlockInterval)

	rootHash := result.InstructionHashes

	// ------------------------------------------------------------------------
	// 2. Invoke a first "addProof"
	// ------------------------------------------------------------------------

	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err := signer.Sign(rootHash[0]) // == index
	require.Nil(t, err)

	index := uint32(0)
	indexBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	proposedTransaction.Instructions[0].SignerIdentities = append(proposedTransaction.Instructions[0].SignerIdentities, identity)
	proposedTransaction.Instructions[0].Signatures = append(proposedTransaction.Instructions[0].Signatures, signature)

	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 2)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].SignerIdentities), 1)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[0].SignerIdentities[0]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].Signatures), 1)
	require.Equal(t, result.ProposedTransaction.Instructions[0].Signatures[0], signature)
	// Default MaxNumExecution should be 1
	require.Equal(t, result.MaxNumExecution, uint64(1))

	require.NotEmpty(t, result.InstructionHashes)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 3. Invoke a second "addProof" (second instruction)
	// ------------------------------------------------------------------------

	signature, err = signer.Sign(rootHash[1]) // == index
	require.Nil(t, err)

	index = uint32(1)
	indexBuf = make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{4},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	proposedTransaction.Instructions[1].SignerIdentities = append(proposedTransaction.Instructions[1].SignerIdentities, identity)
	proposedTransaction.Instructions[1].Signatures = append(proposedTransaction.Instructions[1].Signatures, signature)

	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 2)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[1].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[1].SignerIdentities), 1)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[1].SignerIdentities[0]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[1].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[1].Signatures), 1)
	require.Equal(t, result.ProposedTransaction.Instructions[1].Signatures[0], signature)
	// Default MaxNumExecution should be 1
	require.Equal(t, result.MaxNumExecution, uint64(1))

	require.NotEmpty(t, result.InstructionHashes)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 4. Invoke an "execRoot" command
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
			Command:    "execProposedTx",
		},
		SignerCounter: []uint64{5},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	local.WaitDone(genesisMsg.BlockInterval)

}

func TestDeferred_DefaultExpireBlockIdx(t *testing.T) {
	// Here we invoke a deferred contract without giving an expire block index.
	// We expect then the block index to be the default value we use, which is
	// `current_blockIdx + 50`. In this case, current_blockIdx equals 0.

	// ------------------------------------------------------------------------
	// 0. Set up
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
	require.NoError(t, err)
	require.NoError(t, cl.UseNode(0))

	expectedBlockIdx := uint64(50)

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	rootInstructionValue := []byte("aef123456789fab")

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
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
	})
	require.NoError(t, err)

	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireBlockIndex, expectedBlockIdx)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	local.WaitDone(genesisMsg.BlockInterval)
}

func TestDeferred_ScenarioUpdateConfig(t *testing.T) {
	// In this test we use Invoke:config.update_config as the proposed
	// transaction. We update the config and check if the changes are applied.
	//
	// 1. Spawn a new contract with config as the deferred transaction
	// 2. Invoke an "addProff"
	// 3. Invoke an "execRoot"

	// ------------------------------------------------------------------------
	// 0. Set up
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
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------

	// Get the latest chain config
	prr, err := cl.GetProofFromLatest(byzcoin.ConfigInstanceID.Slice())
	require.Nil(t, err)
	proof := prr.Proof

	_, value, _, _, err := proof.KeyValue()
	require.Nil(t, err)
	config := byzcoin.ChainConfig{}
	err = protobuf.DecodeWithConstructors(value, &config, network.DefaultConstructors(cothority.Suite))
	require.Nil(t, err)
	config.BlockInterval, err = time.ParseDuration("7s")
	require.Nil(t, err)
	config.MaxBlockSize += 10

	configBuf, err := protobuf.Encode(&config)
	require.Nil(t, err)

	invoke := byzcoin.Invoke{
		ContractID: byzcoin.ContractConfigID,
		Command:    "update_config",
		Args: []byzcoin.Argument{
			{
				Name:  "config",
				Value: configBuf,
			},
		},
	}

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.ConfigInstanceID,
		Invoke:     &invoke,
	})
	require.NoError(t, err)

	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	rootHash := result.InstructionHashes

	// ------------------------------------------------------------------------
	// 2.1 Invoke a first "addProof"
	// ------------------------------------------------------------------------

	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err := signer.Sign(rootHash[0]) // == index
	require.Nil(t, err)
	// signature[1] = 0xf

	index := uint32(0)
	indexBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	proposedTransaction.Instructions[0].SignerIdentities = append(proposedTransaction.Instructions[0].SignerIdentities, identity)
	proposedTransaction.Instructions[0].Signatures = append(proposedTransaction.Instructions[0].Signatures, signature)

	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].SignerIdentities), 1)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[0].SignerIdentities[0]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].Signatures), 1)
	require.Equal(t, result.ProposedTransaction.Instructions[0].Signatures[0], signature)
	// Default MaxNumExecution should be 1
	require.Equal(t, result.MaxNumExecution, uint64(1))

	require.NotEmpty(t, result.InstructionHashes)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// ------------------------------------------------------------------------
	// 3. Invoke an "execRoot" command
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
			Command:    "execProposedTx",
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	result, err = cl.GetDeferredData(myID)
	require.Equal(t, 1, len(result.ExecResult))

	time.Sleep(2 * genesisMsg.BlockInterval)
	pr, err := cl.WaitProof(byzcoin.NewInstanceID(byzcoin.ConfigInstanceID.Slice()), 2*genesisMsg.BlockInterval, nil)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(byzcoin.ConfigInstanceID.Slice()))

	_, valueBuf, _, _, err := pr.KeyValue()
	require.Nil(t, err)

	configResult := byzcoin.ChainConfig{}
	err = protobuf.Decode(valueBuf, &configResult)
	require.Nil(t, err)

	// We check if what we get has the updated values
	require.Equal(t, config.BlockInterval, configResult.BlockInterval)
	require.Equal(t, config.MaxBlockSize, configResult.MaxBlockSize)
	require.Equal(t, config.Roster, configResult.Roster)
	require.Equal(t, config.DarcContractIDs, configResult.DarcContractIDs)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))
}

func TestDeferred_ScenarioMultipleSigners(t *testing.T) {
	// The plan is to update a value contract for which two identities must be
	// used. Here is what we do:
	//
	// 1. Spawn a value contract
	// 2. Spawn a deferred contract
	// 3. Add proof with the first signer
	// 4. Try to execute the proposed transaction
	// 5. Add proof with the second signer
	// 6. Execute the proposed transaction
	// 7. Try to execute the proposed transaction a second time

	// ------------------------------------------------------------------------
	// 0. Set up
	// ------------------------------------------------------------------------
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	signer2 := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:value", "spawn:deferred", "invoke:deferred.execProposedTx"},
		signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc
	require.NoError(t, gDarc.Rules.AddRule(darc.Action("invoke:value.update"),
		expression.InitAndExpr(signer.Identity().String(), signer2.Identity().String())))
	require.NoError(t, gDarc.Rules.AddRule(darc.Action("invoke:deferred.addProof"),
		expression.InitOrExpr(signer.Identity().String(), signer2.Identity().String())))

	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn the value contract
	// ------------------------------------------------------------------------

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

	valueID := ctx.Instructions[0].DeriveID("")

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)
	pr, err := cl.WaitProof(byzcoin.NewInstanceID(valueID.Slice()), 2*genesisMsg.BlockInterval, myvalue)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(valueID.Slice()))
	v0, _, _, err := pr.Get(valueID.Slice())
	require.Nil(t, err)
	require.Equal(t, myvalue, v0)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// ------------------------------------------------------------------------
	// 2. Spawn the deferred contract with a value contract update as the
	//    proposed transaction.
	// ------------------------------------------------------------------------
	updatedValue := []byte("aef123456789fab")

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: valueID,
		Invoke: &byzcoin.Invoke{
			ContractID: "value",
			Command:    "update",
			Args: byzcoin.Arguments{
				byzcoin.Argument{
					Name:  "value",
					Value: updatedValue,
				},
			},
		},
	})
	require.NoError(t, err)

	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
			},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	rootHash := result.InstructionHashes

	// ------------------------------------------------------------------------
	// 3. Invoke a first "addProof" with the first signer
	// ------------------------------------------------------------------------

	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err := signer.Sign(rootHash[0]) // == index
	require.Nil(t, err)
	// signature[1] = 0xf

	index := uint32(0)
	indexBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	proposedTransaction.Instructions[0].SignerIdentities = append(proposedTransaction.Instructions[0].SignerIdentities, identity)
	proposedTransaction.Instructions[0].Signatures = append(proposedTransaction.Instructions[0].Signatures, signature)

	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].SignerIdentities), 1)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[0].SignerIdentities[0]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].Signatures), 1)
	require.Equal(t, result.ProposedTransaction.Instructions[0].Signatures[0], signature)
	// Default MaxNumExecution should be 1
	require.Equal(t, result.MaxNumExecution, uint64(1))

	require.NotEmpty(t, result.InstructionHashes)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// ------------------------------------------------------------------------
	// 4. Try to exec the proposed transaction. Should fail since only one
	//    signer has signed.
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
			Command:    "execProposedTx",
		},
		SignerCounter: []uint64{4},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// ------------------------------------------------------------------------
	// 5. Invoke a second "addProof" with the second signer
	// ------------------------------------------------------------------------

	identity = signer2.Identity()
	identityBuf, err = protobuf.Encode(&identity)
	require.Nil(t, err)

	signature, err = signer2.Sign(rootHash[0]) // == index
	require.Nil(t, err)
	// signature[1] = 0xf // Simulates a wrong signature

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
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
				{
					Name:  "index",
					Value: indexBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer2))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	proposedTransaction.Instructions[0].SignerIdentities = append(proposedTransaction.Instructions[0].SignerIdentities, identity)
	proposedTransaction.Instructions[0].Signatures = append(proposedTransaction.Instructions[0].Signatures, signature)

	result, err = cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction.Instructions.Hash(), proposedTransaction.Instructions.Hash())
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].SignerIdentities), 2)
	// This test won't work. But by using Equal() will.
	// require.Equal(t, result.ProposedTransaction.Instructions[0].SignerIdentities[0], identity)
	require.True(t, identity.Equal(&result.ProposedTransaction.Instructions[0].SignerIdentities[1]))

	require.NotEmpty(t, result.ProposedTransaction.Instructions[0].Signatures)
	require.Equal(t, len(result.ProposedTransaction.Instructions[0].Signatures), 2)
	require.Equal(t, result.ProposedTransaction.Instructions[0].Signatures[1], signature)

	require.NotEmpty(t, result.InstructionHashes)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// ------------------------------------------------------------------------
	// 6. Invoke an "execRoot" command
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
			Command:    "execProposedTx",
		},
		SignerCounter: []uint64{4},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	result, err = cl.GetDeferredData(myID)
	require.Equal(t, 1, len(result.ExecResult))

	time.Sleep(2 * genesisMsg.BlockInterval)
	pr, err = cl.WaitProof(byzcoin.NewInstanceID(valueID.Slice()), 2*genesisMsg.BlockInterval, nil)
	require.Nil(t, err)
	require.True(t, pr.InclusionProof.Match(valueID.Slice()))

	valueRes, _, _, err := pr.Get(valueID.Slice())
	require.Nil(t, err)

	// Such a miracle to retrieve the updated value
	require.Equal(t, valueRes, updatedValue)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// ------------------------------------------------------------------------
	// 7. Invoke an "execRoot" command a second time. Since MaxNumExecution should
	//    be at 0, we expect it to fail.
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDeferredID,
			Command:    "execProposedTx",
		},
		SignerCounter: []uint64{5},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))
}

func TestDeferred_SimpleDelete(t *testing.T) {
	// We spawn a deferred instance, delete it and check if is has been
	// correctly deleted.
	//
	// 1. Spawn a new contract
	// 2. Delete the contract

	// ------------------------------------------------------------------------
	// 0. Set up
	// ------------------------------------------------------------------------
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:deferred", "delete:deferred"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	rootInstructionValue := []byte("aef123456789fab")

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
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
	})
	require.NoError(t, err)

	expireBlockIndexInt := uint64(6000)
	expireBlockIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(expireBlockIndexBuf, expireBlockIndexInt)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
				{
					Name:  "expireBlockIndex",
					Value: expireBlockIndexBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// ------------------------------------------------------------------------
	// 2. Invoke a delete
	// ------------------------------------------------------------------------

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Delete: &byzcoin.Delete{
			ContractID: byzcoin.ContractDeferredID,
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	prr, err := cl.GetProofFromLatest(myID.Slice())
	require.Nil(t, err)
	exist, err := prr.Proof.InclusionProof.Exists(myID.Slice())
	require.Nil(t, err)
	require.False(t, exist)
}

func TestDeferred_PublicDelete(t *testing.T) {
	// We spawn a deferred instance with an expire block index at 0, which
	// allows anyone to delete it. We then use a new signer that has no rights
	// to delete the deferred instance.
	//
	// 1. Spawn a new contract
	// 2. Delete the contract with a new signer

	// ------------------------------------------------------------------------
	// 0. Set up
	// ------------------------------------------------------------------------
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:deferred", "delete:deferred"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)
	require.NoError(t, cl.UseNode(0))

	// ------------------------------------------------------------------------
	// 1. Spawn
	// ------------------------------------------------------------------------
	rootInstructionValue := []byte("aef123456789fab")

	proposedTransaction, err := cl.CreateTransaction(byzcoin.Instruction{
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
	})
	require.NoError(t, err)

	expireBlockIndexInt := uint64(0)
	expireBlockIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(expireBlockIndexBuf, expireBlockIndexInt)
	proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
	require.Nil(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDeferredID,
			Args: []byzcoin.Argument{
				{
					Name:  "proposedTransaction",
					Value: proposedTransactionBuf,
				},
				{
					Name:  "expireBlockIndex",
					Value: expireBlockIndexBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	result, err := cl.GetDeferredData(myID)
	require.Nil(t, err)

	require.Equal(t, result.ProposedTransaction, proposedTransaction)
	require.Equal(t, len(result.ProposedTransaction.Instructions), 1)
	require.Equal(t, result.ExpireBlockIndex, expireBlockIndexInt)
	require.Empty(t, result.ProposedTransaction.Instructions[0].SignerIdentities)
	require.Empty(t, result.ProposedTransaction.Instructions[0].Signatures)

	local.WaitDone(genesisMsg.BlockInterval)

	// ------------------------------------------------------------------------
	// 2. Invoke a delete with a new signer that has no rights. It should
	//    work since we allow anyone to delete an expired deferred instance.
	// ------------------------------------------------------------------------

	signer2 := darc.NewSignerEd25519(nil, nil)

	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Delete: &byzcoin.Delete{
			ContractID: byzcoin.ContractDeferredID,
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer2))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Nil(t, err)

	prr, err := cl.GetProofFromLatest(myID.Slice())
	require.Nil(t, err)
	exist, err := prr.Proof.InclusionProof.Exists(myID.Slice())
	require.Nil(t, err)
	require.False(t, exist)
}
