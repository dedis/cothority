package bevm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/c4dt/go-ethereum/common"
	"github.com/c4dt/go-ethereum/core"
	"github.com/c4dt/go-ethereum/core/state"
	"github.com/c4dt/go-ethereum/core/types"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// ContractBEvmID identifies the ByzCoin contract that handles Ethereum contracts
var ContractBEvmID = "bevm"

// ContractBEvmValueID identifies the ByzCoin contract that handles EVM state database values
var ContractBEvmValueID = "bevm_value"

var nilAddress = common.HexToAddress("0x0000000000000000000000000000000000000000")

// ByzCoin contract state for BEvm
type contractBEvm struct {
	byzcoin.BasicContract
	State
}

// ByzCoin contract state for BEvm values
// This contract is only a byproduct of BEvm state changes; it does not support Spawn or Invoke
type contractBEvmValue struct {
	byzcoin.BasicContract
}

// Deserialize a BEvm contract state
func contractBEvmFromBytes(in []byte) (byzcoin.Contract, error) {
	contract := &contractBEvm{}

	err := protobuf.Decode(in, &contract.State)
	if err != nil {
		return nil, err
	}

	return contract, nil
}

// State is the BEvm main contract persisted information, able to handle the EVM state database
type State struct {
	RootHash common.Hash // Hash of the last commit in the EVM state database
	KeyList  []string    // List of keys contained in the EVM state database
}

// NewEvmDb creates a new EVM state database from the contract state
func NewEvmDb(es *State, roStateTrie byzcoin.ReadOnlyStateTrie, instanceID byzcoin.InstanceID) (*state.StateDB, error) {
	byzDb, err := NewServerByzDatabase(instanceID, es.KeyList, roStateTrie)
	if err != nil {
		return nil, err
	}

	db := state.NewDatabase(byzDb)

	return state.New(es.RootHash, db)
}

// NewContractState create a new contract state from the EVM state database
func NewContractState(stateDb *state.StateDB) (*State, []byzcoin.StateChange, error) {
	// Commit the underlying databases first
	root, err := stateDb.Commit(true)
	if err != nil {
		return nil, nil, err
	}

	err = stateDb.Database().TrieDB().Commit(root, true)
	if err != nil {
		return nil, nil, err
	}

	// Retrieve the low-level database
	byzDb, ok := stateDb.Database().TrieDB().DiskDB().(*ServerByzDatabase)
	if !ok {
		return nil, nil, errors.New("Internal error: EVM State DB is not of expected type")
	}

	// Dump the low-level database contents changes
	stateChanges, keyList, err := byzDb.Dump()
	if err != nil {
		return nil, nil, err
	}

	// Build the new EVM state
	return &State{RootHash: root, KeyList: keyList}, stateChanges, nil
}

// DeleteValues returns a list of state changes to delete all the values in the EVM state database
func DeleteValues(keyList []string, stateDb *state.StateDB) ([]byzcoin.StateChange, error) {
	// Retrieve the low-level database
	byzDb, ok := stateDb.Database().TrieDB().DiskDB().(*ServerByzDatabase)
	if !ok {
		return nil, errors.New("Internal error: EVM State DB is not of expected type")
	}

	// Delete all the values
	for _, key := range keyList {
		err := byzDb.Delete([]byte(key))
		if err != nil {
			return nil, err
		}
	}

	// Dump the low-level database contents changes
	stateChanges, keyList, err := byzDb.Dump()
	if err != nil {
		return nil, err
	}

	// Sanity check: the resulted list of keys should be empty
	if len(keyList) != 0 {
		return nil, errors.New("Internal error: DeleteValues() does not produce an empty key list")
	}

	return stateChanges, nil
}

// Spawn creates a new BEvm contract
func (c *contractBEvm) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	// Convention for newly-spawned instances
	instanceID := inst.DeriveID("")

	stateDb, err := NewEvmDb(&c.State, rst, instanceID)
	if err != nil {
		return nil, nil, err
	}

	contractState, _, err := NewContractState(stateDb)
	if err != nil {
		return nil, nil, err
	}

	contractData, err := protobuf.Encode(contractState)
	if err != nil {
		return nil, nil, err
	}
	// State changes to ByzCoin contain a single Create
	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, instanceID, ContractBEvmID, contractData, darc.ID(inst.InstanceID.Slice())),
	}

	return
}

// Helper function to check that all required arguments are provided
func checkArguments(inst byzcoin.Instruction, names ...string) error {
	for _, name := range names {
		if inst.Invoke.Args.Search(name) == nil {
			return fmt.Errorf("Missing '%s' argument", name)
		}
	}

	return nil
}

// Invoke calls a method on an existing BEvm contract
func (c *contractBEvm) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	stateDb, err := NewEvmDb(&c.State, rst, inst.InstanceID)
	if err != nil {
		return nil, nil, err
	}

	switch inst.Invoke.Command {
	case "credit": // Credit an Ethereum account
		err := checkArguments(inst, "address", "amount")
		if err != nil {
			return nil, nil, err
		}

		address := common.BytesToAddress(inst.Invoke.Args.Search("address"))
		amount := new(big.Int).SetBytes(inst.Invoke.Args.Search("amount"))

		stateDb.AddBalance(address, amount)

		contractState, stateChanges, err := NewContractState(stateDb)
		if err != nil {
			return nil, nil, err
		}

		contractData, err := protobuf.Encode(contractState)
		if err != nil {
			return nil, nil, err
		}

		// State changes to ByzCoin contain the Update to the main contract state, plus whatever changes
		// were produced by the EVM on its state database.
		sc = append([]byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID, ContractBEvmID, contractData, darcID),
		}, stateChanges...)

	case "transaction": // Perform an Ethereum transaction (contract method call with state change)
		err := checkArguments(inst, "tx")
		if err != nil {
			return nil, nil, err
		}

		var ethTx types.Transaction
		err = ethTx.UnmarshalJSON(inst.Invoke.Args.Search("tx"))
		if err != nil {
			return nil, nil, err
		}

		txReceipt, err := sendTx(&ethTx, stateDb)
		if err != nil {
			return nil, nil, err
		}

		if txReceipt.ContractAddress.Hex() != nilAddress.Hex() {
			log.Lvlf2("Contract deployed at '%s'", txReceipt.ContractAddress.Hex())
		} else {
			log.Lvlf2("Transaction to '%s'", ethTx.To().Hex())
		}
		log.Lvlf2("\\--> status = %d, gas used = %d, receipt = %s",
			txReceipt.Status, txReceipt.GasUsed, txReceipt.TxHash.Hex())

		contractState, stateChanges, err := NewContractState(stateDb)
		if err != nil {
			return nil, nil, err
		}

		contractData, err := protobuf.Encode(contractState)
		if err != nil {
			return nil, nil, err
		}

		// State changes to ByzCoin contain the Update to the main contract state, plus whatever changes
		// were produced by the EVM on its state database.
		sc = append([]byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID, ContractBEvmID, contractData, darcID),
		}, stateChanges...)

	default:
		err = fmt.Errorf("Unknown Invoke command: '%s'", inst.Invoke.Command)
	}

	return
}

// Helper function that sends a transaction to the EVM
func sendTx(tx *types.Transaction, stateDb *state.StateDB) (*types.Receipt, error) {

	// Gets parameters defined in params
	chainConfig := getChainConfig()
	vmConfig := getVMConfig()

	// GasPool tracks the amount of gas available during execution of the transactions in a block
	gp := new(core.GasPool).AddGas(uint64(1e18))
	usedGas := uint64(0)
	ug := &usedGas

	// ChainContext supports retrieving headers and consensus parameters from the
	// current blockchain to be used during transaction processing.
	var bc core.ChainContext

	// Header represents a block header in the Ethereum blockchain.
	var header *types.Header
	header = &types.Header{
		Number:     big.NewInt(0),
		Difficulty: big.NewInt(0),
		ParentHash: common.Hash{0},
		Time:       0,
	}

	// Apply transaction to the general EVM state
	receipt, usedGas, err := core.ApplyTransaction(chainConfig, bc, &nilAddress, gp, stateDb, header, tx, ug, vmConfig)
	if err != nil {
		return nil, err
	}

	return receipt, nil
}

// Delete deletes an existing BEvm contract
func (c *contractBEvm) Delete(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	stateDb, err := NewEvmDb(&c.State, rst, inst.InstanceID)
	if err != nil {
		return nil, nil, err
	}

	stateChanges, err := DeleteValues(c.State.KeyList, stateDb)
	if err != nil {
		return nil, nil, err
	}

	// State changes to ByzCoin contain the Delete of the main contract state,
	// plus the Delete of all the BEvmValue contracts known to it.
	sc = append([]byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Remove, inst.InstanceID, ContractBEvmID, nil, darcID),
	}, stateChanges...)

	return
}
