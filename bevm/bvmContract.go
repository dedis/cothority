package bevm

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"math/big"
)

var ContractBvmID = "bvm"
var nilAddress = common.HexToAddress("0x0000000000000000000000000000000000000000")

type contractBvm struct {
	byzcoin.BasicContract
	ES
}

func contractBvmFromBytes(in []byte) (byzcoin.Contract, error) {
	cv := &contractBvm{}
	err := protobuf.Decode(in, &cv.ES)
	if err != nil {
		return nil, err
	}
	return cv, nil
}

//Spawn deploys an EVM
func (c *contractBvm) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	evmDb, err := NewEvmDb(&c.ES)
	if err != nil {
		return nil, nil, err
	}

	newEvmState, err := evmDb.getNewEvmState()
	if err != nil {
		return nil, nil, err
	}

	// Then create a StateChange request with the data of the instance. The
	// InstanceID is given by the DeriveID method of the instruction that allows
	// to create multiple instanceIDs out of a given instruction in a pseudo-
	// random way that will be the same for all nodes.
	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""), ContractBvmID, newEvmState, darc.ID(inst.InstanceID.Slice())),
	}
	/*
		for i, sc := range sc{
			log.Printf("state-change %d is %x", i, sha256.Sum256(sc.Value))
		}*/
	return
}

//Invoke provides three instructions : display, credit and transaction
func (c *contractBvm) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	evmDb, err := NewEvmDb(&c.ES)
	if err != nil {
		return nil, nil, err
	}

	switch inst.Invoke.Command {
	case "display":
		addressBuf := inst.Invoke.Args.Search("address")
		if addressBuf == nil {
			return nil, nil, errors.New("no address provided")
		}
		address := common.BytesToAddress(addressBuf)

		balance := evmDb.stateDb.GetBalance(address)
		if balance == big.NewInt(0) {
			log.LLvl1(address.Hex(), "balance", "0")
		}
		log.LLvl1(address.Hex(), "balance", balance.Uint64(), "wei")

	case "credit":
		addressBuf := inst.Invoke.Args.Search("address")
		if addressBuf == nil {
			return nil, nil, errors.New("no address provided")
		}
		address := common.BytesToAddress(addressBuf)

		amountBuf := inst.Invoke.Args.Search("amount")
		if amountBuf == nil {
			return nil, nil, errors.New("no amount provided")
		}
		amount := new(big.Int).SetBytes(amountBuf)
		evmDb.stateDb.AddBalance(address, amount)
		log.Lvl1("balance set to", amount, "wei")

		newEvmState, err := evmDb.getNewEvmState()
		if err != nil {
			return nil, nil, err
		}

		sc = []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
				ContractBvmID, newEvmState, darcID),
		}

	case "transaction":
		txBuffer := inst.Invoke.Args.Search("tx")
		if txBuffer == nil {
			log.LLvl1("no transaction provided in byzcoin transaction")
			return nil, nil, err
		}
		var ethTx types.Transaction
		err = ethTx.UnmarshalJSON(txBuffer)
		if err != nil {
			return nil, nil, err
		}

		transactionReceipt, err := sendTx(&ethTx, evmDb.stateDb)
		if err != nil {
			log.ErrFatal(err)
			return nil, nil, err
		}

		if transactionReceipt.ContractAddress.Hex() != nilAddress.Hex() {
			log.LLvl1("contract deployed at:", transactionReceipt.ContractAddress.Hex(), "tx status:", transactionReceipt.Status, "gas used:", transactionReceipt.GasUsed, "tx receipt:", transactionReceipt.TxHash.Hex())
		} else {
			log.LLvl1("transaction to", ethTx.To().Hex(), "from", "tx status:", transactionReceipt.Status, "gas used:", transactionReceipt.GasUsed, "tx receipt:", transactionReceipt.TxHash.Hex())
		}

		newEvmState, err := evmDb.getNewEvmState()
		if err != nil {
			return nil, nil, err
		}

		sc = []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
				ContractBvmID, newEvmState, darcID),
		}

	default:
		err = errors.New("Contract can only display, credit and receive transactions")
	}

	return
}

//sendTx is a helper function that applies the signed transaction to a general state
func sendTx(tx *types.Transaction, stateDb *state.StateDB) (*types.Receipt, error) {

	//Gets parameters defined in params
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
	//Applies transaction to the general state
	receipt, usedGas, err := core.ApplyTransaction(chainConfig, bc, &nilAddress, gp, stateDb, header, tx, ug, vmConfig)
	if err != nil {
		log.Error()
		return nil, err
	}
	//logs := stateDb.GetLogs(receipt.TxHash)
	//log.LLvl1("the log we are looking for", stateDb.GetLogs(common.HexToHash("0x47386f9020e5b01eccee6f293ab2b0dc47479f0793e980a4e49bd0c6473e30b1")))
	//log.LLvl1(logs)
	return receipt, nil
}

//EthereumState structure contains DbBuf the general state and RootHash the hash of the last commit
type ES struct {
	DbBuf    []byte
	RootHash common.Hash
}
