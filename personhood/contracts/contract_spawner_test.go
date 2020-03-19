package contracts

import (
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
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
	err = protobuf.Decode(scs[0].Value, cs)
	require.NoError(t, err)
	require.Equal(t, uint64(200), cs.CostDarc.Value)
	require.Equal(t, uint64(200), cs.CostCRead.Value)
	require.Equal(t, uint64(100), cs.CostCWrite.Value)
	require.Equal(t, uint64(200), cs.CostValue.Value)
}

func TestContractSpawnerCoin(t *testing.T) {
	iid := byzcoin.NewInstanceID([]byte("some coin"))
	s := newRstSimul()
	s.values[string(iid.Slice())] = byzcoin.StateChangeBody{}
	cs := &ContractSpawner{}
	cost := byzcoin.Coin{Name: contracts.CoinName, Value: 200}
	costBuf, err := protobuf.Encode(&cost)
	require.NoError(t, err)
	inst := byzcoin.Instruction{
		InstanceID: iid,
		Spawn: &byzcoin.Spawn{
			ContractID: ContractSpawnerID,
			Args: byzcoin.Arguments{
				{Name: "costCoin", Value: costBuf},
			},
		},
	}
	scs, _, err := cs.Spawn(s, inst, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(scs))
	s.Process(scs)

	coin1 := byzcoin.Coin{Name: contracts.CoinName, Value: uint64(1000)}
	err = protobuf.Decode(scs[0].Value, cs)
	require.NoError(t, err)
	var out []byzcoin.Coin
	scs, out, err = cs.Spawn(s, byzcoin.Instruction{
		InstanceID: iid,
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractCoinID,
			Args: []byzcoin.Argument{
				{Name: "coinValue", Value: []byte{100, 0, 0, 0, 0, 0, 0, 0}}},
		}}, []byzcoin.Coin{coin1})
	require.NoError(t, err)
	require.Equal(t, 1, len(scs))
	require.Equal(t, uint64(700), out[0].Value)
	var coinOut byzcoin.Coin
	err = protobuf.Decode(scs[0].Value, &coinOut)
	require.Equal(t, uint64(100), coinOut.Value)

	coin1 = byzcoin.Coin{Name: contracts.CoinName, Value: uint64(300)}
	coin2 := byzcoin.Coin{Name: contracts.CoinName, Value: uint64(100)}
	require.NoError(t, err)
	scs, out, err = cs.Spawn(s, byzcoin.Instruction{
		InstanceID: iid,
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractCoinID,
			Args: []byzcoin.Argument{
				{Name: "coinValue", Value: []byte{200, 0, 0, 0, 0, 0, 0, 0}}},
		}}, []byzcoin.Coin{coin1, coin2})
	require.NoError(t, err)
	require.Equal(t, 1, len(scs))
	require.Equal(t, uint64(0), out[0].Value)
	require.Equal(t, uint64(0), out[1].Value)
	err = protobuf.Decode(scs[0].Value, &coinOut)
	require.Equal(t, uint64(200), coinOut.Value)
}
