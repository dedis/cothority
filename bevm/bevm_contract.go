package bevm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

var ContractBEvmID = "bevm"
var nilAddress = common.HexToAddress("0x0000000000000000000000000000000000000000")

type contractBEvm struct {
	byzcoin.BasicContract
	BEvmState
}

// Deserialize a BEVM contract state
func contractBEvmFromBytes(in []byte) (byzcoin.Contract, error) {
	contract := &contractBEvm{}

	err := protobuf.Decode(in, &contract.BEvmState)
	if err != nil {
		return nil, err
	}

	return contract, nil
}

// Byzcoin EVM State
type BEvmState struct {
	RootHash common.Hash // Hash of the last commit
	KeyList  []string
}

// Create a new EVM state DB from the contract state
func NewEvmDb(es *BEvmState, roStateTrie byzcoin.ReadOnlyStateTrie, instanceID byzcoin.InstanceID) (*state.StateDB, error) {
	byzDb, err := NewServerByzDatabase(es.KeyList, roStateTrie, instanceID)
	if err != nil {
		return nil, err
	}

	db := state.NewDatabase(byzDb)

	return state.New(es.RootHash, db)
}

// Create a new contract state from the EVM state DB
func NewContractState(stateDb *state.StateDB) (*BEvmState, []byzcoin.StateChange, error) {
	// Commit the underlying databases first
	root, err := stateDb.Commit(true)
	if err != nil {
		return nil, nil, err
	}

	err = stateDb.Database().TrieDB().Commit(root, true)
	if err != nil {
		return nil, nil, err
	}

	// Retrieve and dump the low-level database
	byzDb, ok := stateDb.Database().TrieDB().DiskDB().(*ByzDatabase)
	if !ok {
		return nil, nil, errors.New("Internal error: EVM State DB is not of expected type")
	}
	stateChanges, keyList, err := byzDb.Dump()
	if err != nil {
		return nil, nil, err
	}

	// Build the new EVM state
	return &BEvmState{RootHash: root, KeyList: keyList}, stateChanges, nil
}

// Spawn a new BEVM contract
func (c *contractBEvm) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	instanceID := inst.DeriveID("")

	stateDb, err := NewEvmDb(&c.BEvmState, rst, instanceID)
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
	// Then create a StateChange request with the data of the instance. The
	// InstanceID is given by the DeriveID method of the instruction that allows
	// to create multiple instanceIDs out of a given instruction in a pseudo-
	// random way that will be the same for all nodes.
	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, instanceID, ContractBEvmID, contractData, darc.ID(inst.InstanceID.Slice())),
	}

	return
}

// Helper function to check all required arguments are provided
func checkArguments(inst byzcoin.Instruction, names ...string) error {
	for _, name := range names {
		if inst.Invoke.Args.Search(name) == nil {
			return errors.New(fmt.Sprintf("Missing '%s' argument", name))
		}
	}

	return nil
}

func (c *contractBEvm) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	stateDb, err := NewEvmDb(&c.BEvmState, rst, inst.InstanceID)
	if err != nil {
		return nil, nil, err
	}

	switch inst.Invoke.Command {
	case "credit":
		err := checkArguments(inst, "address", "amount")
		if err != nil {
			return nil, nil, err
		}

		address := common.BytesToAddress(inst.Invoke.Args.Search("address"))
		amount := new(big.Int).SetBytes(inst.Invoke.Args.Search("amount"))

		stateDb.AddBalance(address, amount)
		log.Lvl1("balance set to", amount, "wei")

		contractState, stateChanges, err := NewContractState(stateDb)
		if err != nil {
			return nil, nil, err
		}

		contractData, err := protobuf.Encode(contractState)
		if err != nil {
			return nil, nil, err
		}

		sc = append([]byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
				ContractBEvmID, contractData, darcID),
		}, stateChanges...)

	case "transaction":
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
			log.ErrFatal(err)
			return nil, nil, err
		}

		if txReceipt.ContractAddress.Hex() != nilAddress.Hex() {
			log.LLvl1("contract deployed at:", txReceipt.ContractAddress.Hex(), "tx status:", txReceipt.Status, "gas used:", txReceipt.GasUsed, "tx receipt:", txReceipt.TxHash.Hex())
		} else {
			log.LLvl1("transaction to", ethTx.To().Hex(), "from", "tx status:", txReceipt.Status, "gas used:", txReceipt.GasUsed, "tx receipt:", txReceipt.TxHash.Hex())
		}

		contractState, stateChanges, err := NewContractState(stateDb)
		if err != nil {
			return nil, nil, err
		}

		contractData, err := protobuf.Encode(contractState)
		if err != nil {
			return nil, nil, err
		}

		sc = append([]byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
				ContractBEvmID, contractData, darcID),
		}, stateChanges...)

	default:
		err = errors.New(fmt.Sprintf("Unknown Invoke command: '%s'", inst.Invoke.Command))
	}

	return
}

// Helper function that applies the signed EVM transaction to a general state
func sendTx(tx *types.Transaction, stateDb *state.StateDB) (*types.Receipt, error) {

	// Gets parameters defined in params
	chainConfig := getChainConfig()
	vmConfig := getVMConfig()

	// GasPool tracks the amount of gas available during execution of the transactions in a block.
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
		Time:       big.NewInt(0),
	}

	// Apply transaction to the general state
	receipt, usedGas, err := core.ApplyTransaction(chainConfig, bc, &nilAddress, gp, stateDb, header, tx, ug, vmConfig)
	if err != nil {
		log.Error()
		return nil, err
	}

	return receipt, nil
}
