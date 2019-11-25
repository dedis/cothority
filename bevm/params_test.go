package bevm

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"go.dedis.ch/onet/v3/log"
)

func newEvmMemDb() (*state.StateDB, error) {
	buffer := []byte{}

	memDb, err := NewMemDatabase(buffer)
	if err != nil {
		return nil, err
	}

	var root common.Hash
	db := state.NewDatabase(memDb)

	return state.New(root, db)
}

// TestTokenContract verifies that the EVM setup is correct by testing a dummy tokenContract
func TestTokenContract(t *testing.T) {

	// Parameters functions
	canTransfer := func(vm.StateDB, common.Address, *big.Int) bool { return true }
	transfer := func(vm.StateDB, common.Address, common.Address, *big.Int) {}
	getHash := func(uint64) common.Hash { return common.HexToHash("O") }

	// Get smart contract abi and bytecode
	contract, err := NewEvmContract(getContractPath(t, "ModifiedToken"))
	require.Nil(t, err)

	// Create dummy addresses for testing token transfers
	addressA := common.HexToAddress("a")
	addressB := common.HexToAddress("b")

	accountRef := vm.AccountRef(common.HexToAddress("0"))

	// Helper functions for contract calls, uses the abi object defined above

	// Constructor (function create), mints 12 tokens and credits them to public key A
	create, err := contract.Abi.Pack("create", uint64(12), addressA)
	require.Nil(t, err)

	// Get balance function to check if tokens where indeed credited
	get, err := contract.Abi.Pack("getBalance", addressA)
	require.Nil(t, err)

	// Transfer function, with parameters to transfer from addressA to addressB, one token
	send, err := contract.Abi.Pack("transfer", addressA, addressB, uint64(1))
	require.Nil(t, err)

	// Get balance function to check if token was indeed credited
	getB, err := contract.Abi.Pack("getBalance", addressB)
	require.Nil(t, err)
	// Get balance function to check if token was indeed credited
	getA, err := contract.Abi.Pack("getBalance", addressA)
	require.Nil(t, err)

	// Transfer function, with parameters to transfer from addressA to addressB, one more token
	transferTests, err := contract.Abi.Pack("transfer", addressA, addressB, uint64(1))
	require.Nil(t, err)

	// Empty general Ethereum state database to instantiate EVM
	sdb, err := newEvmMemDb()
	require.Nil(t, err)

	// Context for instantiating EVM
	ctx := vm.Context{CanTransfer: canTransfer, Transfer: transfer, GetHash: getHash, Origin: addressA, GasPrice: big.NewInt(1), Coinbase: addressA, GasLimit: 10000000000, BlockNumber: big.NewInt(0), Time: big.NewInt(1), Difficulty: big.NewInt(1)}

	// Set up the Byzcoin Virtual Machine, a copy of EVM with our parameters
	bevm := vm.NewEVM(ctx, sdb, getChainConfig(), getVMConfig())

	// Contract deployment
	_, addrContract, leftOverGas, err := bevm.Create(accountRef, contract.Bytecode, 100000000, big.NewInt(0))
	require.Nil(t, err)

	// Call the methods from the contract we just deployed using the abi helpers function defined above
	// Constructor (create method) call
	_, _, err = bevm.Call(accountRef, addrContract, create, leftOverGas, big.NewInt(0))
	require.Nil(t, err)

	var balance uint64

	// Get balance of addressA
	retBalanceOfAccountA, _, err := bevm.Call(accountRef, addrContract, get, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	err = contract.Abi.Unpack(&balance, "getBalance", retBalanceOfAccountA)
	require.Nil(t, err)
	log.Lvl2(addressA.Hex(), "address, token balance :", balance)

	// Send a token from A to B
	_, _, err = bevm.Call(accountRef, addrContract, send, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	log.Lvl2("send one token from", addressA.Hex(), " to ", addressB.Hex())

	// Get balance of addressA
	retBalanceOfAccountA, _, err = bevm.Call(accountRef, addrContract, getA, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	err = contract.Abi.Unpack(&balance, "getBalance", retBalanceOfAccountA)
	require.Nil(t, err)
	log.Lvl2(addressA.Hex(), "address, token balance :", balance)
	require.Equal(t, balance, uint64(11))

	// Check if the other account was updated accordingly
	retBalanceOfAccountB, _, err := bevm.Call(accountRef, addrContract, getB, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	err = contract.Abi.Unpack(&balance, "getBalance", retBalanceOfAccountB)
	require.Nil(t, err)
	log.Lvl2(addressB.Hex(), "address, token balance :", balance)
	require.Equal(t, balance, uint64(1))

	// Transfer one more token
	_, _, err = bevm.Call(accountRef, addrContract, transferTests, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	log.Lvl2("send one token from", addressA.Hex(), " to ", addressB.Hex())

	// Get balance of addressA
	retBalanceOfAccountA, _, err = bevm.Call(accountRef, addrContract, getA, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	err = contract.Abi.Unpack(&balance, "getBalance", retBalanceOfAccountA)
	require.Nil(t, err)
	log.Lvl2(addressA.Hex(), "address, token balance :", balance)
	require.Equal(t, balance, uint64(10))

	// Get balance of addressB
	retBalanceOfAccountB, _, err = bevm.Call(accountRef, addrContract, getB, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	err = contract.Abi.Unpack(&balance, "getBalance", retBalanceOfAccountB)
	require.Nil(t, err)
	log.Lvl2(addressB.Hex(), "address, token balance :", balance)
	require.Equal(t, balance, uint64(2))
}
