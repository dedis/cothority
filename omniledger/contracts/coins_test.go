package contracts

import (
	"testing"

	"github.com/dedis/cothority/omniledger/collection"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/stretchr/testify/require"
)

var coinZero, coinOne, coinTwo []byte

func init() {
	coinZero = make([]byte, 8)
	coinOne = make([]byte, 8)
	coinOne[0] = 1
	coinTwo = make([]byte, 8)
	coinTwo[0] = 2
}

func TestCoin_Spawn(t *testing.T) {
	// Testing spawning of a new coin and checking it has zero coins in it.
	ct := cvTest{}
	inst := omniledger.Instruction{
		InstanceID: omniledger.NewInstanceID(nil),
		Spawn: &omniledger.Spawn{
			ContractID: ContractCoinID,
		},
	}

	c := []omniledger.Coin{}
	sc, co, err := ContractCoin(ct, inst, c)
	require.Nil(t, err)
	require.Equal(t, 1, len(sc))
	ca := omniledger.InstanceID{DarcID: make([]byte, 32), SubID: omniledger.NewSubID(inst.Hash())}
	require.Equal(t, omniledger.NewStateChange(omniledger.Create, ca,
		ContractCoinID, coinZero), sc[0])
	require.Equal(t, 0, len(co))
}

func TestCoin_InvokeMint(t *testing.T) {
	// Test that a coin can be minted
	ct := newCT()
	coAddr := omniledger.NewInstanceID(nil)
	ct.Store(coAddr, coinZero, ContractCoinID)

	inst := omniledger.Instruction{
		InstanceID: coAddr,
		Invoke: &omniledger.Invoke{
			Command: "mint",
			Args:    omniledger.Arguments{{Name: "coins", Value: coinOne}},
		},
	}
	sc, co, err := ContractCoin(ct, inst, []omniledger.Coin{})
	require.Nil(t, err)
	require.Equal(t, 0, len(co))
	require.Equal(t, 1, len(sc))
	require.Equal(t, omniledger.NewStateChange(omniledger.Update, coAddr, ContractCoinID, coinOne),
		sc[0])
}

func TestCoin_InvokeOverflow(t *testing.T) {
	uint64max := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	ct := newCT()
	coAddr := omniledger.NewInstanceID(nil)
	ct.Store(coAddr, uint64max, ContractCoinID)

	inst := omniledger.Instruction{
		InstanceID: coAddr,
		Invoke: &omniledger.Invoke{
			Command: "mint",
			Args:    omniledger.Arguments{{Name: "coins", Value: coinOne}},
		},
	}
	sc, co, err := ContractCoin(ct, inst, []omniledger.Coin{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "overflow")
	require.Equal(t, 0, len(co))
	require.Equal(t, 0, len(sc))
}

func TestCoin_InvokeStoreFetch(t *testing.T) {
	ct := newCT()
	coAddr := omniledger.NewInstanceID(nil)
	ct.Store(coAddr, coinZero, ContractCoinID)

	inst := omniledger.Instruction{
		InstanceID: coAddr,
		Invoke: &omniledger.Invoke{
			Command: "store",
			Args:    nil,
		},
	}
	c1 := omniledger.Coin{Name: CoinName, Value: 1}
	notOlCoin := iid("notOlCoin")
	c2 := omniledger.Coin{Name: notOlCoin, Value: 1}

	sc, co, err := ContractCoin(ct, inst, []omniledger.Coin{c1, c2})
	require.Nil(t, err)
	require.Equal(t, 1, len(co))
	require.Equal(t, co[0].Name, notOlCoin)
	require.Equal(t, 1, len(sc))
	require.Equal(t, omniledger.NewStateChange(omniledger.Update, coAddr, ContractCoinID, coinOne),
		sc[0])

	inst = omniledger.Instruction{
		InstanceID: coAddr,
		Invoke: &omniledger.Invoke{
			Command: "fetch",
			Args:    omniledger.Arguments{{Name: "coins", Value: coinOne}},
		},
	}

	// Try once with not enough coins available.
	sc, co, err = ContractCoin(ct, inst, nil)
	require.Error(t, err)

	// Apply the changes to the mock collection.
	ct.Store(coAddr, coinOne, ContractCoinID)

	sc, co, err = ContractCoin(ct, inst, nil)
	require.Nil(t, err)
	require.Equal(t, 1, len(co))
	require.Equal(t, co[0].Name, CoinName)
	require.Equal(t, uint64(1), co[0].Value)
	require.Equal(t, 1, len(sc))
	require.Equal(t, omniledger.NewStateChange(omniledger.Update, coAddr, ContractCoinID, coinZero),
		sc[0])
}

func TestCoin_InvokeTransfer(t *testing.T) {
	// Test that a coin can be transferred
	ct := newCT()
	coAddr1 := omniledger.InstanceID{DarcID: make([]byte, 32), SubID: omniledger.SubID{}}
	coAddr2 := omniledger.InstanceID{DarcID: make([]byte, 32), SubID: omniledger.SubID{}}
	coAddr2.DarcID[31] = byte(1)
	ct.Store(coAddr1, coinOne, ContractCoinID)
	ct.Store(coAddr2, coinZero, ContractCoinID)

	// First create an instruction where the transfer should fail
	inst := omniledger.Instruction{
		InstanceID: coAddr2,
		Invoke: &omniledger.Invoke{
			Command: "transfer",
			Args: omniledger.Arguments{
				{Name: "coins", Value: coinOne},
				{Name: "destination", Value: coAddr1.Slice()},
			},
		},
	}
	sc, co, err := ContractCoin(ct, inst, []omniledger.Coin{})
	require.Error(t, err)

	inst = omniledger.Instruction{
		InstanceID: coAddr1,
		Invoke: &omniledger.Invoke{
			Command: "transfer",
			Args: omniledger.Arguments{
				{Name: "coins", Value: coinOne},
				{Name: "destination", Value: coAddr2.Slice()},
			},
		},
	}
	sc, co, err = ContractCoin(ct, inst, []omniledger.Coin{})
	require.Nil(t, err)
	require.Equal(t, 0, len(co))
	require.Equal(t, 2, len(sc))
	require.Equal(t, omniledger.NewStateChange(omniledger.Update, coAddr2, ContractCoinID, coinOne), sc[0])
	require.Equal(t, omniledger.NewStateChange(omniledger.Update, coAddr1, ContractCoinID, coinZero), sc[1])
}

type cvTest struct {
	values      map[string][]byte
	contractIDs map[string]string
}

func newCT() *cvTest {
	return &cvTest{make(map[string][]byte), make(map[string]string)}
}

func (ct cvTest) Get(key []byte) collection.Getter {
	panic("not implemented")
}
func (ct *cvTest) Store(key omniledger.InstanceID, value []byte, contractID string) {
	k := string(key.Slice())
	ct.values[k] = value
	ct.contractIDs[k] = contractID
}
func (ct cvTest) GetValues(key []byte) (value []byte, contractID string, err error) {
	return ct.values[string(key)], ct.contractIDs[string(key)], nil
}
func (ct cvTest) GetValue(key []byte) ([]byte, error) {
	return ct.values[string(key)], nil
}
func (ct cvTest) GetContractID(key []byte) (string, error) {
	return ct.contractIDs[string(key)], nil
}
