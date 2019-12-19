package contracts

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/protobuf"
)

func TestContractSpawner(t *testing.T) {
	iid := byzcoin.NewInstanceID([]byte("some coin"))
	s := newRstSimul()
	s.values[string(iid.Slice())] = byzcoin.StateChangeBody{}
	cs := &ContractSpawner{}
	cost := byzcoin.Coin{Name: iid, Value: 200}
	costBuf, err := protobuf.Encode(&cost)
	require.NoError(t, err)
	inst := byzcoin.Instruction{
		InstanceID: iid,
		Spawn: &byzcoin.Spawn{
			ContractID: ContractSpawnerID,
			Args: byzcoin.Arguments{
				{Name: "costDarc", Value: costBuf},
				{Name: "costCRead", Value: costBuf},
				{Name: "costRoPaSci", Value: costBuf},
				{Name: "costValue", Value: costBuf},
			},
		},
	}
	scs, _, err := cs.Spawn(s, inst, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(scs))
	spawner := &SpawnerStruct{}
	err = protobuf.Decode(scs[0].Value, spawner)
	require.NoError(t, err)
	require.Equal(t, uint64(200), spawner.CostDarc.Value)
	require.Equal(t, uint64(200), spawner.CostCRead.Value)
	require.Equal(t, uint64(100), spawner.CostCWrite.Value)
	require.Equal(t, uint64(200), spawner.CostValue.Value)
}
