package byzcoin

import (
	"errors"
	"github.com/stretchr/testify/require"
	"math/big"
	"strings"
	"testing"

	"go.dedis.ch/onet/v3/log"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// TestTokenContract verifies that the EVM setup is correct by testing a dummy tokenContract
func TestTokenContract(t *testing.T) {

	//Parameters functions
	canTransfer := func(vm.StateDB, common.Address, *big.Int) bool {return true}
	transfer := func(vm.StateDB, common.Address, common.Address, *big.Int) {}
	getHash := func(uint64) common.Hash {return common.HexToHash("O")}

	//Get smart contract abi and bytecode
	simpleAbi, simpleBin := getSmartContract( "ModifiedToken")

	//Create dummy addresses for testing token transfers
	addressA := common.HexToAddress("a")
	addressB := common.HexToAddress("b")


	accountRef := vm.AccountRef(common.HexToAddress("0"))


	//Cration of the abi object
	abi, err := abi.JSON(strings.NewReader(simpleAbi))
	require.Nil(t, err)

	//Helper functions for contract calls, uses the abi object defined above

	//Constructor (function create), mints 12 tokens and credits them to public key A
	create, err := abi.Pack("create", uint64(12), addressA)
	require.Nil(t, err)

	//Get balance function to check if tokens where indeed credited
	get, err := abi.Pack("getBalance", addressA)
	require.Nil(t, err)

	//Transfer function, with parameters to transfer from addressA to addressB, one token
	send, err := abi.Pack("transfer", addressA, addressB,uint64(1))
	require.Nil(t, err)

	//Get balance function to check if token was indeed credited
	get1, err := abi.Pack("getBalance", addressB)
	require.Nil(t, err)
	//Get balance function to check if token was indeed credited
	//Get balance function to check if token was indeed credited
	get2, err := abi.Pack("getBalance", addressA)
	require.Nil(t, err)

	//Transfer function, with parameters to transfer from addressA to addressB, one more token
	transferTests, err := abi.Pack("transfer", addressA, addressB, uint64(1))
	require.Nil(t, err)

	//Empty general Ethereum state database to instantiate EVM
	_, sdb, err := getDB(ES{DbBuf: []byte{}})
	require.Nil(t, err)

	//Context for instantiating EVM
	ctx := vm.Context{CanTransfer: canTransfer, Transfer: transfer, GetHash: getHash, Origin: addressA, GasPrice: big.NewInt(1), Coinbase: addressA, GasLimit: 10000000000, BlockNumber: big.NewInt(0), Time: big.NewInt(1), Difficulty: big.NewInt(1)}

	//Setting up the Byzcoin Virtual Machine, a copy of EVM with our parameters
	bvm := vm.NewEVM(ctx, sdb, getChainConfig(), getVMConfig())


	//Contract deployment
	retContractCreation, addrContract, leftOverGas, err := bvm.Create(accountRef, common.Hex2Bytes(simpleBin), 100000000, big.NewInt(0))
	if err != nil {
		err = errors.New("contract deployment unsuccessful")
		log.LLvl1("return of contract creation", common.Bytes2Hex(retContractCreation))
		log.ErrFatal(err)
	}
	log.LLvl1("contract deployed at", addrContract.Hex())


	//Calling the methods from the contract we just deployed using the abi helpers function defined above
	//Constructor (create method) call
	_, _, err = bvm.Call(accountRef, addrContract, create, leftOverGas, big.NewInt(0))
	require.Nil(t, err)

	//Getting balance of addressA
	retBalanceOfAccountA, _, err := bvm.Call(accountRef, addrContract, get, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	log.Lvl1( addressA.Hex(),"address, token balance :", retBalanceOfAccountA)

	//Sending a token from A to B
	_, _, err = bvm.Call(accountRef, addrContract, send, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	log.Lvl1("send one token from", addressA.Hex(), " to ", addressB.Hex())

	//Getting balance of addressA
	retBalanceOfAccountB, _, err := bvm.Call(accountRef, addrContract, get1, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	log.Lvl1( addressB.Hex(),"address, token balance :", retBalanceOfAccountB)

	//Checking if the other account was updated accordingly
	retBalanceOfAccountA, _, err = bvm.Call(accountRef, addrContract, get2, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	log.Lvl1( addressA.Hex(),"address, token balance :", retBalanceOfAccountA)

	//Transfering one more token
	_, _, err = bvm.Call(accountRef, addrContract, transferTests, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	log.Lvl1("send one token from", addressA.Hex(), " to ", addressB.Hex())

	//Getting balance of addressB
	retBalanceOfAccountB, _, err = bvm.Call(accountRef, addrContract, get1, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	log.Lvl1( addressB.Hex(),"address, token balance :", retBalanceOfAccountB)

	//Getting balance of addressA
	retBalanceOfAccountA, _, err = bvm.Call(accountRef, addrContract, get2, leftOverGas, big.NewInt(0))
	require.Nil(t, err)
	log.Lvl1("balance of ", addressA.Hex(), " is ", retBalanceOfAccountA)
	log.LLvl1("contract calls passed")
}



