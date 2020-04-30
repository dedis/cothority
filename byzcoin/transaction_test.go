package byzcoin

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin/trie"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/protobuf"
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
	require.NoError(t, err)

	// create a db/trie
	mdb := trie.NewMemDB()
	tr, err := trie.NewTrie(mdb, []byte("my nonce"))
	require.NoError(t, err)
	sst := &stagingStateTrie{*tr.MakeStagingTrie(), trieCache{}}

	// verification should fail because trie is empty
	ctxHash := ctx.Instructions.Hash()
	require.NoError(t, err)
	require.Error(t, ctx.Instructions[0].Verify(sst, ctxHash))
	require.Contains(t, ctx.Instructions[0].Verify(sst, ctxHash).Error(), "key not set")

	// set the version, but it's too high, verification should fail
	require.NoError(t, setSignerCounter(sst, signer.Identity().String(), 10))
	require.Error(t, ctx.Instructions[0].Verify(sst, ctxHash))
	require.Contains(t, ctx.Instructions[0].Verify(sst, ctxHash).Error(), "got counter=")

	// set the config
	config := ChainConfig{
		DarcContractIDs: []string{"darc"},
	}
	configBuf, err := protobuf.Encode(&config)
	require.NoError(t, err)
	err = sst.StoreAll([]StateChange{
		{
			InstanceID:  NewInstanceID(nil).Slice(),
			StateAction: Create,
			ContractID:  ContractConfigID,
			Value:       configBuf,
		},
	})
	require.NoError(t, err)

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
		ContractID:  ContractDarcID,
		Value:       darcBuf,
		DarcID:      d.GetBaseID(),
	}
	require.NoError(t, sst.StoreAll([]StateChange{sc}))
	require.NoError(t, ctx.Instructions[0].Verify(sst, ctxHash))
}

func TestTransactionBuffer_Add(t *testing.T) {
	b := newTxBuffer()
	key := "abc"
	key2 := "abcd"

	for i := 0; i < defaultMaxBufferSize*2; i++ {
		b.add(key, ClientTransaction{})
		b.add(key2, ClientTransaction{})
	}

	require.Equal(t, defaultMaxBufferSize, len(b.txsMap[key]))
	require.Equal(t, defaultMaxBufferSize, len(b.txsMap[key2]))
}

func TestTransactionBuffer_Take(t *testing.T) {
	b := newTxBuffer()
	key := "abc"

	for i := 0; i < 100; i++ {
		b.add(key, ClientTransaction{})
	}

	txs := b.take(key, 12)
	require.Equal(t, 12, len(txs))
	require.Equal(t, 88, len(b.txsMap[key]))

	txs = b.take(key, 100)
	require.Equal(t, 88, len(txs))
	_, ok := b.txsMap[key]
	require.False(t, ok)

	txs = b.take(key, 100)
	require.Equal(t, 0, len(txs))
}

func TestTransactionBuffer_TakeDisabled(t *testing.T) {
	b := newTxBuffer()
	key := "abc"

	for i := 0; i < 10; i++ {
		b.add(key, ClientTransaction{})
	}

	txs := b.take(key, -1)
	require.Equal(t, 10, len(txs))
	_, ok := b.txsMap[key]
	require.False(t, ok)
}

func TestInstruction_DeriveIDArg(t *testing.T) {
	inst := Instruction{
		InstanceID: NewInstanceID([]byte("new instance")),
	}
	_, err := inst.DeriveIDArg("", "testID")
	require.Error(t, err)
	inst.Spawn = &Spawn{ContractID: "test"}
	id1, err := inst.DeriveIDArg("", "testID")
	require.NoError(t, err)
	inst.Spawn.Args = Arguments{{Name: "testID", Value: []byte("testing")}}
	id2, err := inst.DeriveIDArg("", "testID")
	require.NoError(t, err)
	require.NotEqual(t, id1, id2)
}

func setSignerCounter(sst *stagingStateTrie, id string, v uint64) error {
	key := publicVersionKey(id)
	verBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(verBuf, v)
	body := StateChangeBody{
		StateAction: Update,
		ContractID:  "",
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
	instr := createSpawnInstr(dID, kind, "data", value)
	instr.SignerCounter = []uint64{counter}
	instr.SignerIdentities = []darc.Identity{signer.Identity()}
	t := ClientTransaction{
		Instructions: []Instruction{instr},
	}
	h := t.Instructions.Hash()
	for i := range t.Instructions {
		if err := t.Instructions[i].SignWith(h, signer); err != nil {
			return t, err
		}
	}
	return t, nil
}

func createClientTxWithTwoInstrWithCounter(dID darc.ID, kind string, value []byte, signer darc.Signer, counter uint64) (ClientTransaction, error) {
	instr1 := createSpawnInstr(dID, kind, "", value)
	instr1.SignerIdentities = []darc.Identity{signer.Identity()}
	instr1.SignerCounter = []uint64{counter}
	instr2 := createSpawnInstr(dID, kind, "", value)
	instr2.SignerIdentities = []darc.Identity{signer.Identity()}
	instr2.SignerCounter = []uint64{counter + 1}
	t := ClientTransaction{
		Instructions: []Instruction{instr1, instr2},
	}
	h := t.Instructions.Hash()
	for i := range t.Instructions {
		if err := t.Instructions[i].SignWith(h, signer); err != nil {
			return t, err
		}
	}
	return t, nil
}

func createSpawnInstr(dID darc.ID, contractID string, argName string, value []byte) Instruction {
	return Instruction{
		InstanceID: NewInstanceID(dID),
		Spawn: &Spawn{
			ContractID: contractID,
			Args:       Arguments{{Name: argName, Value: value}},
		},
		SignerCounter: []uint64{1},
		version:       CurrentVersion,
	}
}

func createInvokeInstr(dID InstanceID, contractID, cmd, argName string, value []byte) Instruction {
	return Instruction{
		InstanceID: dID,
		Invoke: &Invoke{
			ContractID: contractID,
			Command:    cmd,
			Args:       Arguments{{Name: argName, Value: value}},
		},
		version: CurrentVersion,
	}
}

func combineInstrsAndSign(signer darc.Signer, instrs ...Instruction) (ClientTransaction, error) {
	for i := range instrs {
		instrs[i].SignerIdentities = []darc.Identity{signer.Identity()}
	}
	t := NewClientTransaction(CurrentVersion, instrs...)
	h := t.Instructions.Hash()
	for i := range t.Instructions {
		if err := t.Instructions[i].SignWith(h, signer); err != nil {
			return t, err
		}
	}
	return t, nil
}
