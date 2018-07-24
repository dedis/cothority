package service

import (
	"testing"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/stretchr/testify/require"
)

func id(s string) InstanceID {
	return InstanceIDFromSlice([]byte(s))
}

func TestSortTransactions(t *testing.T) {
	ts1 := []ClientTransaction{
		{
			Instructions: []Instruction{{
				InstanceID: id("nonce1"),
				Spawn: &Spawn{
					ContractID: "kind1",
				},
			}}},
		{
			Instructions: []Instruction{{
				InstanceID: id("nonce2"),
				Spawn: &Spawn{
					ContractID: "kind2",
				},
			}}},
		{
			Instructions: []Instruction{{
				InstanceID: id("nonce3"),
				Spawn: &Spawn{
					ContractID: "kind3",
				},
			}}},
	}
	ts2 := []ClientTransaction{
		{
			Instructions: []Instruction{{
				InstanceID: id("nonce2"),
				Spawn: &Spawn{
					ContractID: "kind2",
				},
			}}},
		{
			Instructions: []Instruction{{
				InstanceID: id("nonce1"),
				Spawn: &Spawn{
					ContractID: "kind1",
				},
			}}},
		{
			Instructions: []Instruction{{
				InstanceID: id("nonce3"),
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
	ids := []darc.Identity{signer.Identity()}
	d := darc.NewDarc(darc.InitRules(ids, ids), []byte("genesis darc"))
	d.Rules.AddRule("spawn:dummy_kind", d.Rules.GetSignExpr())
	require.Nil(t, d.Verify(true))

	instr, err := createInstr(d.GetBaseID(), "dummy_kind", []byte("dummy_value"), signer)
	require.Nil(t, err)

	require.Nil(t, instr.SignBy(d.GetBaseID(), signer))

	req, err := instr.ToDarcRequest(d.GetBaseID())
	require.Nil(t, err)
	require.Nil(t, req.Verify(d))
}

func createOneClientTx(dID darc.ID, kind string, value []byte, signer darc.Signer) (ClientTransaction, error) {
	instr, err := createInstr(dID, kind, value, signer)
	if err != nil {
		return ClientTransaction{}, err
	}
	t := ClientTransaction{
		Instructions: []Instruction{instr},
	}
	return t, err
}

func createInstr(dID darc.ID, contractID string, value []byte, signer darc.Signer) (Instruction, error) {
	instr := Instruction{
		InstanceID: InstanceIDFromSlice(dID),
		Spawn: &Spawn{
			ContractID: contractID,
			Args:       Arguments{{Name: "data", Value: value}},
		},
		Nonce:  GenNonce(),
		Index:  0,
		Length: 1,
	}
	err := instr.SignBy(dID, signer)
	return instr, err
}
