package contracts

import (
	"crypto/sha256"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/calypso"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3/log"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/protobuf"
)

func TestContractSpawner(t *testing.T) {
	s := byzcoin.NewROSTSimul()
	iid := byzcoin.NewInstanceID([]byte("some coin"))
	s.Values[string(iid.Slice())] = byzcoin.StateChangeBody{}
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
	s := byzcoin.NewROSTSimul()
	s.Values[string(iid.Slice())] = byzcoin.StateChangeBody{}
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
	_, err = s.StoreAllToReplica(scs)
	require.NoError(t, err)

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

func TestContractSpawnerPreID(t *testing.T) {
	sr := newSpawnerROST(t)
	var null byzcoin.InstanceID

	instSpawner := byzcoin.Instruction{
		// This is bogus, as the verification method is not called, it will
		// just be ignored
		InstanceID: sr.coinID,
		Spawn: &byzcoin.Spawn{
			ContractID: ContractSpawnerID,
		},
	}
	testPreID(t, sr, instSpawner, null, null, false)

	d := darc.NewDarc(darc.NewRules(), []byte("test"))
	dBuf, err := protobuf.Encode(d)
	baseID := byzcoin.NewInstanceID(d.GetBaseID())
	require.NoError(t, err)
	instDarc := byzcoin.Instruction{
		// This is bogus, as the verification method is not called, it will
		// just be ignored
		InstanceID: sr.coinID,
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDarcID,
			Args:       byzcoin.Arguments{{Name: "darc", Value: dBuf}},
		},
	}
	testPreID(t, sr, instDarc, baseID, baseID, false)

	coinID := []byte("coinID")
	instCoin := byzcoin.Instruction{
		// This is bogus, as the verification method is not called, it will
		// just be ignored
		InstanceID: sr.coinID,
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractCoinID,
			Args: byzcoin.Arguments{
				{Name: "coinName", Value: []byte("somecoin")},
				{Name: "coinID", Value: coinID}},
		},
	}
	h := sha256.New()
	h.Write([]byte(contracts.ContractCoinID))
	h.Write(coinID)
	ca := byzcoin.NewInstanceID(h.Sum(nil))
	testPreID(t, sr, instCoin, ca, null, false)

	credID := []byte("credID")
	credBuf, err := protobuf.Encode(&CredentialStruct{})
	instCred := byzcoin.Instruction{
		// This is bogus, as the verification method is not called, it will
		// just be ignored
		InstanceID: sr.coinID,
		Spawn: &byzcoin.Spawn{
			ContractID: ContractCredentialID,
			Args: byzcoin.Arguments{
				{Name: "credential", Value: credBuf},
				{Name: "credID", Value: credID}},
		},
	}
	h = sha256.New()
	h.Write([]byte("credential"))
	h.Write(credID)
	ca = byzcoin.NewInstanceID(h.Sum(nil))
	testPreID(t, sr, instCred, ca, null, false)

	ltsID := byzcoin.NewInstanceID(random.Bits(256, true, random.New()))
	ltsKP := key.NewKeyPair(cothority.Suite)
	wr := calypso.NewWrite(cothority.Suite, ltsID,
		darc.ID{}, ltsKP.Public, []byte("key"))
	wrBuf, err := protobuf.Encode(wr)
	require.NoError(t, err)
	instCalWrite := byzcoin.Instruction{
		// This is bogus, as the verification method is not called, it will
		// just be ignored
		InstanceID: sr.coinID,
		Spawn: &byzcoin.Spawn{
			ContractID: calypso.ContractWriteID,
			Args:       byzcoin.Arguments{{Name: "write", Value: wrBuf}},
		},
	}
	wr4, _ := testPreID(t, sr, instCalWrite, null, null, true)

	rd := &calypso.Read{
		Write: wr4,
		Xc:    ltsKP.Public,
	}
	rdBuf, err := protobuf.Encode(rd)
	require.NoError(t, err)
	instCalRead := byzcoin.Instruction{
		// This is bogus, as the verification method is not called, it will
		// just be ignored
		InstanceID: wr4,
		Spawn: &byzcoin.Spawn{
			ContractID: calypso.ContractReadID,
			Args:       byzcoin.Arguments{{Name: "read", Value: rdBuf}},
		},
	}
	testPreID(t, sr, instCalRead, null, null, true)

	descBuf, err := protobuf.Encode(&PopDesc{})
	require.NoError(t, err)
	something := []byte("somethin")
	instPopP := byzcoin.Instruction{
		// This is bogus, as the verification method is not called, it will
		// just be ignored
		InstanceID: sr.coinID,
		Spawn: &byzcoin.Spawn{
			ContractID: ContractPopPartyID,
			Args: byzcoin.Arguments{
				{Name: "description", Value: descBuf},
				{Name: "darcID", Value: baseID[:]},
				{Name: "miningReward", Value: something},
			},
		},
	}
	testPreID(t, sr, instPopP, null, null, true)

	rpsBuf, err := protobuf.Encode(&RoPaSciStruct{
		FirstPlayerHash: baseID[:],
	})
	require.NoError(t, err)
	instRPS := byzcoin.Instruction{
		// This is bogus, as the verification method is not called, it will
		// just be ignored
		InstanceID: sr.coinID,
		Spawn: &byzcoin.Spawn{
			ContractID: ContractRoPaSciID,
			Args: byzcoin.Arguments{
				{Name: "struct", Value: rpsBuf},
			},
		},
	}
	testPreID(t, sr, instRPS, null, null, true)

	instValue := byzcoin.Instruction{
		// This is bogus, as the verification method is not called, it will
		// just be ignored
		InstanceID: sr.coinID,
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractValueID,
			Args: byzcoin.Arguments{
				{Name: "value", Value: something},
			},
		},
	}
	testPreID(t, sr, instValue, null, null, false)
}

// preIDBeforeVersion5 indicates that the contract does already interpret
// preID before the version 5, which might mean trouble...
func testPreID(t *testing.T, sr spawnerROST,
	inst byzcoin.Instruction, v4Hash, v5Hash byzcoin.InstanceID,
	preIDBeforeVersion5 bool) (v4, v5 byzcoin.InstanceID) {
	if v4Hash.Equal(byzcoin.InstanceID{}) {
		if preIDBeforeVersion5 {
			v4Hash = inst.DeriveID("")
		}
	}

	// First try new version
	sr.Version = byzcoin.VersionPreID
	preID := byzcoin.NewInstanceID([]byte(inst.Spawn.ContractID))
	inst.Spawn.Args = append(inst.Spawn.Args, byzcoin.Argument{
		Name:  "preID",
		Value: preID[:],
	})
	if v4Hash.Equal(byzcoin.InstanceID{}) {
		v4Hash = inst.DeriveID("")
	}
	if v5Hash.Equal(byzcoin.InstanceID{}) {
		var err error
		v5Hash, err = inst.DeriveIDArg("", "preID")
		require.NoError(t, err)
	}
	sc, _ := sr.spawn(t, inst)
	require.Equal(t, 1, len(sc))
	iid := byzcoin.NewInstanceID(sc[0].InstanceID)
	if !v4Hash.Equal(v5Hash) {
		require.NotEqual(t, v4Hash, iid)
	}
	require.Equal(t, v5Hash, iid)
	_, err := sr.StoreAllToReplica(sc)
	require.NoError(t, err)

	// Then try old version
	sr.Version = byzcoin.VersionPreID - 1
	if preIDBeforeVersion5 {
		inst.Spawn.Args = inst.Spawn.Args[:len(inst.Spawn.Args)-1]
	}
	sc, _ = sr.spawn(t, inst)
	require.Equal(t, 1, len(sc))
	iid = byzcoin.NewInstanceID(sc[0].InstanceID)
	require.Equal(t, v4Hash, iid)
	if !v4Hash.Equal(v5Hash) {
		require.NotEqual(t, v5Hash, iid)
	}
	_, err = sr.StoreAllToReplica(sc)
	require.NoError(t, err)

	return v4Hash, v5Hash
}

type spawnerROST struct {
	*byzcoin.ROSTSimul
	coinID byzcoin.InstanceID
	coin   byzcoin.Coin
	coins  []byzcoin.Coin
	cs     ContractSpawner
}

func newSpawnerROST(t *testing.T) (sr spawnerROST) {
	sr.ROSTSimul = byzcoin.NewROSTSimul()
	sr.coinID = byzcoin.NewInstanceID([]byte("some coin"))
	sr.coin = byzcoin.Coin{Name: sr.coinID, Value: 1e6}
	sr.coins = []byzcoin.Coin{sr.coin}
	sr.Values[string(sr.coinID.Slice())] = byzcoin.
		StateChangeBody{ContractID: contracts.ContractCoinID}
	cost := byzcoin.Coin{Name: sr.coinID, Value: 200}
	costBuf, err := protobuf.Encode(&cost)
	require.NoError(t, err)
	inst := byzcoin.Instruction{
		InstanceID: sr.coinID,
		Spawn: &byzcoin.Spawn{
			ContractID: ContractSpawnerID,
			Args: byzcoin.Arguments{
				{Name: "costDarc", Value: costBuf},
				{Name: "costCoin", Value: costBuf},
				{Name: "costCredential", Value: costBuf},
				{Name: "costParty", Value: costBuf},
				{Name: "costRoPaSci", Value: costBuf},
				{Name: "costCWrite", Value: costBuf},
				{Name: "costCRead", Value: costBuf},
				{Name: "costValue", Value: costBuf},
			},
		},
	}
	_, _, err = sr.cs.Spawn(sr, inst, nil)
	require.NoError(t, err)
	return
}

func (sr *spawnerROST) spawn(t *testing.T, inst byzcoin.Instruction) (
	sc []byzcoin.StateChange, cout []byzcoin.Coin) {
	var err error
	cid := sr.Values[string(inst.InstanceID[:])].ContractID
	coins := []byzcoin.Coin{{Name: sr.coin.Name, Value: 1000}}
	switch cid {
	case ContractSpawnerID, contracts.ContractCoinID:
		sc, cout, err = sr.cs.Spawn(sr, inst, coins)
		require.NoError(t, err)
	case calypso.ContractWriteID:
		sc, cout, err = calypso.ContractWrite{}.Spawn(sr, inst, coins)
		require.NoError(t, err)
	default:
		log.Error("don't know this contract: " + cid)
		t.Fail()
	}
	return
}
