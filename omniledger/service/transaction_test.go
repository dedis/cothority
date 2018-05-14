package service

import (
	"testing"

	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"github.com/stretchr/testify/require"
)

func TestSortTransactions(t *testing.T) {
	ts1 := []ClientTransaction{{
		Instructions: []Instruction{{
			DarcID:  []byte("key1"),
			Nonce:   []byte("nonce1"),
			Command: "Create",
			Kind:    "kind1",
			Data:    []byte("value1"),
		}}},
		{
			Instructions: []Instruction{{
				DarcID:  []byte("key2"),
				Nonce:   []byte("nonce2"),
				Command: "Create",
				Kind:    "kind2",
				Data:    []byte("value2"),
			}}},
		{
			Instructions: []Instruction{{
				DarcID:  []byte("key2"),
				Nonce:   []byte("nonce2"),
				Command: "Create",
				Kind:    "kind2",
				Data:    []byte("value2"),
			}}},
	}
	ts2 := []ClientTransaction{{
		Instructions: []Instruction{{
			DarcID:  []byte("key2"),
			Nonce:   []byte("nonce2"),
			Command: "Create",
			Kind:    "kind2",
			Data:    []byte("value2"),
		}}},
		{
			Instructions: []Instruction{{
				DarcID:  []byte("key1"),
				Nonce:   []byte("nonce1"),
				Command: "Create",
				Kind:    "kind1",
				Data:    []byte("value1"),
			}}},
		{
			Instructions: []Instruction{{
				DarcID:  []byte("key2"),
				Nonce:   []byte("nonce2"),
				Command: "Create",
				Kind:    "kind2",
				Data:    []byte("value2"),
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
	d.Rules.AddRule("Create", d.Rules.GetSignExpr())
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

func createInstr(dID darc.ID, kind string, value []byte, signer *darc.Signer) (Instruction, error) {
	instr := Instruction{
		DarcID:  dID,
		Nonce:   GenNonce(),
		Command: "Create", // TODO what?
		Kind:    kind,
		Data:    value,
	}
	err := instr.SignBy(signer)
	return instr, err
}
