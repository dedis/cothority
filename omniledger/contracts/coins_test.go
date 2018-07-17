package contracts

import (
	"testing"

	"github.com/dedis/cothority/omniledger/collection"
	"github.com/dedis/cothority/omniledger/service"
	"github.com/stretchr/testify/require"
)

var (
	coinZero = make([]byte, 8)
	coinOne  = append(make([]byte, 7), byte(1))
	coinTwo  = append(make([]byte, 7), byte(2))
)

func TestCoin_Spawn(t *testing.T) {
	// Testing spawning of a new coin and checking it has zero coins in it.
	ct := cvTest{}
	inst := service.Instruction{
		InstanceID: service.NewInstanceID(nil),
		Spawn: &service.Spawn{
			ContractID: ContractCoinID,
		},
	}

	c := []service.Coin{}
	sc, co, err := ContractCoin(ct, inst, c)
	require.Nil(t, err)
	require.Equal(t, 1, len(sc))
	ca := service.InstanceID{DarcID: make([]byte, 32), SubID: service.NewSubID(inst.Hash())}
	require.Equal(t, service.NewStateChange(service.Create, ca,
		ContractCoinID, coinZero), sc[0])
	require.Equal(t, 0, len(co))
}

func TestCoin_InvokeMint(t *testing.T) {
	// Test that a coin can be minted
	ct := newCT()
	coAddr := service.NewInstanceID(nil)
	ct.Store(coAddr, coinZero, ContractCoinID)

	inst := service.Instruction{
		InstanceID: coAddr,
		Invoke: &service.Invoke{
			Command: "mint",
			Args:    service.Arguments{{Name: "coins", Value: coinOne}},
		},
	}
	sc, co, err := ContractCoin(ct, inst, []service.Coin{})
	require.Nil(t, err)
	require.Equal(t, 0, len(co))
	require.Equal(t, 1, len(sc))
	require.Equal(t, service.NewStateChange(service.Update, coAddr, ContractCoinID, coinOne),
		sc[0])
}

func TestCoin_InvokeTransfer(t *testing.T) {
	// Test that a coin can be transferred
	ct := newCT()
	coAddr1 := service.InstanceID{DarcID: make([]byte, 32), SubID: service.SubID{}}
	coAddr2 := service.InstanceID{DarcID: make([]byte, 32), SubID: service.SubID{}}
	coAddr2.DarcID[31] = byte(1)
	ct.Store(coAddr1, coinOne, ContractCoinID)
	ct.Store(coAddr2, coinZero, ContractCoinID)

	// First create an instruction where the transfer should fail
	inst := service.Instruction{
		InstanceID: coAddr2,
		Invoke: &service.Invoke{
			Command: "transfer",
			Args: service.Arguments{
				{Name: "coins", Value: coinOne},
				{Name: "destination", Value: coAddr1.Slice()},
			},
		},
	}
	sc, co, err := ContractCoin(ct, inst, []service.Coin{})
	require.NotNil(t, err)

	inst = service.Instruction{
		InstanceID: coAddr1,
		Invoke: &service.Invoke{
			Command: "transfer",
			Args: service.Arguments{
				{Name: "coins", Value: coinOne},
				{Name: "destination", Value: coAddr2.Slice()},
			},
		},
	}
	sc, co, err = ContractCoin(ct, inst, []service.Coin{})
	require.Nil(t, err)
	require.Equal(t, 0, len(co))
	require.Equal(t, 2, len(sc))
	require.Equal(t, service.NewStateChange(service.Update, coAddr2, ContractCoinID, coinOne), sc[0])
	require.Equal(t, service.NewStateChange(service.Update, coAddr1, ContractCoinID, coinZero), sc[1])
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
func (ct *cvTest) Store(key service.InstanceID, value []byte, contractID string) {
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
