package contracts

import (
	"testing"

	"go.dedis.ch/protobuf"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
)

func TestContractCredential_Spawn(t *testing.T) {
	cc := &ContractCredential{}
	rost := byzcoin.NewROSTSimul()
	cred := CredentialStruct{}
	d, err := rost.CreateBasicDarc(nil, "credential")
	require.NoError(t, err)
	inst, err := NewInstructionCredentialSpawn(byzcoin.NewInstanceID(nil), d.GetBaseID(),
		byzcoin.NewInstanceID(nil), cred)
	require.NoError(t, err)
	scs, _, err := cc.Spawn(rost, inst, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(scs))
	cred2 := CredentialStruct{}
	err = protobuf.Decode(scs[0].Value, &cred2)
	require.NoError(t, err)
	require.Equal(t, cred, cred2)
}
