package byzcoin

import (
	"encoding/binary"
	"testing"

	"github.com/dedis/cothority/byzcoin/trie"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/require"
)

func TestTransaction_Signing(t *testing.T) {
	// create the darc
	signer := darc.NewSignerEd25519(nil, nil)
	ids := []darc.Identity{signer.Identity()}
	d := darc.NewDarc(darc.InitRules(ids, ids), []byte("genesis darc"))
	d.Rules.AddRule("spawn:dummy_kind", d.Rules.GetSignExpr())
	require.Nil(t, d.Verify(true))

	// create the tx
	ctx, err := createOneClientTx(d.GetBaseID(), "dummy_kind", []byte("dummy_value"), signer)
	require.Nil(t, err)

	// create a db/trie
	mdb := trie.NewMemDB()
	tr, err := trie.NewTrie(mdb, []byte("my nonce"))
	require.NoError(t, err)
	sst := &stagingStateTrie{*tr.MakeStagingTrie()}

	// verification should fail because trie is empty
	ctxHash := ctx.InstructionsHash
	require.NoError(t, err)
	require.Error(t, ctx.Instructions[0].Verify(sst, ctxHash))
	require.Contains(t, ctx.Instructions[0].Verify(sst, ctxHash).Error(), "key not set")

	// set the version, but it's too high, verification should fail
	require.NoError(t, setSignerCounter(sst, signer.Identity().String(), 10))
	require.Error(t, ctx.Instructions[0].Verify(sst, ctxHash))
	require.Contains(t, ctx.Instructions[0].Verify(sst, ctxHash).Error(), "got version")

	// set the right version, but darc is missing, verification should fail
	require.NoError(t, setSignerCounter(sst, signer.Identity().String(), 0))
	require.Error(t, ctx.Instructions[0].Verify(sst, ctxHash))
	require.Contains(t, ctx.Instructions[0].Verify(sst, ctxHash).Error(), "darc not found")

	// verification should fail if the darc is in, but the version number
	// is wrong
	darcBuf, err := d.ToProto()
	require.NoError(t, err)
	sc := StateChange{
		InstanceID:  d.GetBaseID(),
		StateAction: Create,
		ContractID:  []byte("darc"),
		Value:       darcBuf,
		DarcID:      d.GetBaseID(),
	}
	require.NoError(t, sst.StoreAll([]StateChange{sc}))
	require.NoError(t, ctx.Instructions[0].Verify(sst, ctxHash))
}

func setSignerCounter(sst *stagingStateTrie, id string, v uint64) error {
	key := publicVersionKey(id)
	verBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(verBuf, v)
	body := StateChangeBody{
		StateAction: Update,
		ContractID:  []byte{},
		Value:       verBuf,
		DarcID:      darc.ID([]byte{}),
	}
	buf, err := protobuf.Encode(&body)
	if err != nil {
		return err
	}
	return sst.Set(key, buf)
}

func createOneClientTx(dID darc.ID, kind string, value []byte, signer darc.Signer) (ClientTransaction, error) {
	return createOneClientTxWithCounter(dID, kind, value, signer, 1)
}

func createOneClientTxWithCounter(dID darc.ID, kind string, value []byte, signer darc.Signer, counter uint64) (ClientTransaction, error) {
	instr := createInstr(dID, kind, "data", value)
	instr.SignerCounter = []uint64{counter}
	t := ClientTransaction{
		Instructions: []Instruction{instr},
	}
	t.InstructionsHash = t.Instructions.Hash()
	for i := range t.Instructions {
		if err := t.Instructions[i].SignWith(t.InstructionsHash, signer); err != nil {
			return t, err
		}
	}
	return t, nil
}

func createClientTxWithTwoInstrWithCounter(dID darc.ID, kind string, value []byte, signer darc.Signer, counter uint64) (ClientTransaction, error) {
	instr1 := createInstr(dID, kind, "", value)
	instr1.SignerCounter = []uint64{counter}
	instr2 := createInstr(dID, kind, "", value)
	instr2.SignerCounter = []uint64{counter + 1}
	t := ClientTransaction{
		Instructions: []Instruction{instr1, instr2},
	}
	t.InstructionsHash = t.Instructions.Hash()
	for i := range t.Instructions {
		if err := t.Instructions[i].SignWith(t.InstructionsHash, signer); err != nil {
			return t, err
		}
	}
	return t, nil
}

func createInstr(dID darc.ID, contractID string, argName string, value []byte) Instruction {
	return Instruction{
		InstanceID: NewInstanceID(dID),
		Spawn: &Spawn{
			ContractID: contractID,
			Args:       Arguments{{Name: argName, Value: value}},
		},
		SignerCounter: []uint64{1},
	}
}

func combineInstrsAndSign(signer darc.Signer, instrs ...Instruction) (ClientTransaction, error) {
	t := ClientTransaction{
		Instructions: instrs,
	}
	t.InstructionsHash = t.Instructions.Hash()
	for i := range t.Instructions {
		if err := t.Instructions[i].SignWith(t.InstructionsHash, signer); err != nil {
			return t, err
		}
	}
	return t, nil
}
