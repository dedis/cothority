package bevm

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

type EvmContract struct {
	Abi      abi.ABI
	Bytecode []byte
	Address  common.Address
}

func (contract EvmContract) packConstructor(args ...interface{}) ([]byte, error) {
	return contract.packMethod("", args...)
}

func (contract EvmContract) packMethod(method string, args ...interface{}) ([]byte, error) {
	return contract.Abi.Pack(method, args...)
}

func (contract EvmContract) String() string {
	return fmt.Sprintf("EvmContract@%s", contract.Address.Hex())
}

type EvmAccount struct {
	Address    common.Address
	PrivateKey *ecdsa.PrivateKey
	Nonce      uint64
}

func (account EvmAccount) String() string {
	return fmt.Sprintf("EvmAccount(%s)", account.Address.Hex())
}

func NewEvmAccount(address string, privateKey string) (*EvmAccount, error) {
	key, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, err
	}

	return &EvmAccount{
		Address:    common.HexToAddress(address),
		PrivateKey: key,
	}, nil
}

//returns abi and bytecode of solidity contract
func getSmartContract(nameOfContract string) (*EvmContract, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	contractPath := dir + "/contracts/" + nameOfContract + "/" + nameOfContract + "_sol_" + nameOfContract

	abiJson, err := ioutil.ReadFile(contractPath + ".abi")
	if err != nil {
		return nil, errors.New("Error reading contract ABI: " + err.Error())
	}

	contractAbi, err := abi.JSON(strings.NewReader(string(abiJson)))
	if err != nil {
		return nil, errors.New("Error decoding contract ABI JSON: " + err.Error())
	}

	contractBytecode, err := ioutil.ReadFile(contractPath + ".bin")
	if err != nil {
		return nil, errors.New("Error reading contract Bytecode: " + err.Error())
	}

	return &EvmContract{Abi: contractAbi, Bytecode: common.Hex2Bytes(string(contractBytecode))}, nil
}

func getChainConfig() *params.ChainConfig {
	///ChainConfig (adapted from Rinkeby test net)
	chainconfig := &params.ChainConfig{
		ChainID:             big.NewInt(1),
		HomesteadBlock:      big.NewInt(0),
		DAOForkBlock:        nil,
		DAOForkSupport:      false,
		EIP150Block:         nil,
		EIP150Hash:          common.HexToHash("0x0000000000000000000000000000000000000000"),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: nil,
		Clique: &params.CliqueConfig{
			Period: 15,
			Epoch:  30000,
		},
	}
	return chainconfig
}

func getVMConfig() vm.Config {
	//vmConfig Config
	vmconfig := &vm.Config{
		// Debug enabled debugging Interpreter options
		Debug: false,
		// Tracer is the op code logger
		Tracer: nil,
		// NoRecursion disabled Interpreter call, callcode,
		// delegate call and create.
		NoRecursion: false,
		// Enable recording of SHA3/keccak preimages
		EnablePreimageRecording: true,
		// JumpTable contains the EVM instruction table. This
		// may be left uninitialised and will be set to the default
		// table.
		//JumpTable [256]operation
		//JumpTable: ,
		// Type of the EWASM interpreter
		EWASMInterpreter: "",
		// Type of the EVM interpreter
		EVMInterpreter: "",
	}
	return *vmconfig
}

func returnCanTransfer() func(vm.StateDB, common.Address, *big.Int) bool {
	canTransfer := func(vm.StateDB, common.Address, *big.Int) bool {
		return true
	}
	return canTransfer
}

func returnTransfer() func(vm.StateDB, common.Address, common.Address, *big.Int) {
	transfer := func(vm.StateDB, common.Address, common.Address, *big.Int) {
	}
	return transfer
}

func returnGetHash() func(uint64) common.Hash {
	gethash := func(uint64) common.Hash {
		return common.HexToHash("0")
	}
	return gethash

}

func getContext() vm.Context {
	placeHolder := common.HexToAddress("0")
	return vm.Context{
		CanTransfer: returnCanTransfer(),
		Transfer:    returnTransfer(),
		GetHash:     returnGetHash(),
		Origin:      placeHolder,
		GasPrice:    big.NewInt(0),
		Coinbase:    placeHolder,
		GasLimit:    10000000000,
		BlockNumber: big.NewInt(0),
		Time:        big.NewInt(1),
		Difficulty:  big.NewInt(1),
	}

}

//getDB returns the Memory Database and the general State database given the old Ethereum general state
func getDB(es ES) (*MemDatabase, *state.StateDB, error) {
	memDb, err := NewMemDatabase(es.DbBuf)
	if err != nil {
		return nil, nil, err
	}
	db := state.NewDatabase(memDb)
	stateDb, err := state.New(es.RootHash, db)
	if err != nil {
		return nil, nil, err
	}
	return memDb, stateDb, nil
}

//spawnEvm will return the memory database, the general state database and the EVM on which transactions will be applied
func spawnEvm() (*MemDatabase, *state.StateDB, *vm.EVM, error) {
	memDb, stateDb, err := getDB(ES{DbBuf: []byte{}})
	if err != nil {
		return nil, nil, nil, err
	}
	bvm := vm.NewEVM(getContext(), stateDb, getChainConfig(), getVMConfig())
	return memDb, stateDb, bvm, nil
}
