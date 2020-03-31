package contracts

import (
	"testing"
	"time"

	"go.dedis.ch/protobuf"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
)

// Creates a party, activates the barrier point, finalizes it, and mines the coins.
func TestContractPopParty(t *testing.T) {
	cpp := &ContractPopParty{}
	rost := byzcoin.NewROSTSimul()
	d, err := rost.CreateBasicDarc(nil, "pp")
	require.NoError(t, err)
	desc := PopDesc{
		Name:     "Test",
		Purpose:  "Testing",
		DateTime: uint64(time.Now().UnixNano()),
		Location: "go-test",
	}
	reward := uint64(1234)
	inst, err := NewInstructionPoppartySpawn(byzcoin.NewInstanceID(nil),
		d.GetBaseID(), desc, reward)
	require.NoError(t, err)
	scs, _, err := cpp.Spawn(rost, inst, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(scs))
	pps := PopPartyStruct{}
	err = protobuf.Decode(scs[0].Value, &pps)
	require.NoError(t, err)
	require.Equal(t, desc, pps.Description)
}
