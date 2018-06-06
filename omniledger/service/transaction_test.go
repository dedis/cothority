package service

import (
	"testing"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/stretchr/testify/require"
)

func nonceStr(s string) (n Nonce) {
	copy(n[:], s)
	return n
}
func darcidStr(s string) (n darc.ID) {
	n = make([]byte, 32)
	copy(n, s)
	return n
}

func TestSortTransactions(t *testing.T) {
	ts1 := []ClientTransaction{
		{
			Instructions: []Instruction{{
				ObjectID: ObjectID{
					DarcID:     darcidStr("key1"),
					InstanceID: nonceStr("nonce1"),
				},
				Spawn: &Spawn{
					ContractID: "kind1",
				},
			}}},
		{
			Instructions: []Instruction{{
				ObjectID: ObjectID{
					DarcID:     darcidStr("key2"),
					InstanceID: nonceStr("nonce2"),
				},
				Spawn: &Spawn{
					ContractID: "kind2",
				},
			}}},
		{
			Instructions: []Instruction{{
				ObjectID: ObjectID{
					DarcID:     darcidStr("key3"),
					InstanceID: nonceStr("nonce3"),
				},
				Spawn: &Spawn{
					ContractID: "kind3",
				},
			}}},
	}
	ts2 := []ClientTransaction{
		{
			Instructions: []Instruction{{
				ObjectID: ObjectID{
					DarcID:     darcidStr("key2"),
					InstanceID: nonceStr("nonce2"),
				},
				Spawn: &Spawn{
					ContractID: "kind2",
				},
			}}},
		{
			Instructions: []Instruction{{
				ObjectID: ObjectID{
					DarcID:     darcidStr("key1"),
					InstanceID: nonceStr("nonce1"),
				},
				Spawn: &Spawn{
					ContractID: "kind1",
				},
			}}},
		{
			Instructions: []Instruction{{
				ObjectID: ObjectID{
					DarcID:     darcidStr("key3"),
					InstanceID: nonceStr("nonce3"),
				},
				Spawn: &Spawn{
					ContractID: "kind3",
				},
			}}},
	}
	err := sortTransactions(ts1)
	require.Nil(t, err)
	err = sortTransactions(ts2)
	require.Nil(t, err)
	for i := range ts1 {
		require.Equal(t, ts1[i], ts2[i])
	}
}

func TestTransaction_Signing(t *testing.T) {
	signer := darc.NewSignerEd25519(nil, nil)
	ids := []*darc.Identity{signer.Identity()}
	d := darc.NewDarc(darc.InitRules(ids, ids), []byte("genesis darc"))
	d.Rules.AddRule("Spawn_dummy_kind", d.Rules.GetSignExpr())
	require.Nil(t, d.Verify())

	instr, err := createInstr(d.GetBaseID(), "dummy_kind", []byte("dummy_value"), signer)
	require.Nil(t, err)

	require.Nil(t, instr.SignBy(signer))

	req, err := instr.ToDarcRequest()
	require.Nil(t, err)
	require.Nil(t, req.Verify(d))
}

func createOneClientTx(dID darc.ID, kind string, value []byte, signer *darc.Signer) (ClientTransaction, error) {
	instr, err := createInstr(dID, kind, value, signer)
	t := ClientTransaction{
		Instructions: []Instruction{instr},
	}
	return t, err
}

func createInstr(dID darc.ID, contractID string, value []byte, signer *darc.Signer) (Instruction, error) {
	instr := Instruction{
		ObjectID: ObjectID{
			DarcID:     dID,
			InstanceID: GenNonce(),
		},
		Spawn: &Spawn{
			ContractID: contractID,
			Args:       Arguments{{Name: "data", Value: value}},
		},
	}
	err := instr.SignBy(signer)
	return instr, err
}
