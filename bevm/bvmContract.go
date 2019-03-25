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
	es := c.ES
	memdb, db, _, err := spawnEvm()
	if err != nil {
		return nil, nil, err
	}
	es.RootHash, err = db.Commit(true)
	if err != nil {
		return nil, nil, err
	}
	err = db.Database().TrieDB().Commit(es.RootHash, true)
	if err != nil {
		return nil, nil, err
	}
	es.DbBuf, err = memdb.Dump()
	esBuf, err := protobuf.Encode(&es)
	// Then create a StateChange request with the data of the instance. The
	// InstanceID is given by the DeriveID method of the instruction that allows
	// to create multiple instanceIDs out of a given instruction in a pseudo-
	// random way that will be the same for all nodes.
	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""), ContractBvmID, esBuf, darc.ID(inst.InstanceID.Slice())),
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
	es := c.ES
	switch inst.Invoke.Command {

	case "display":
		addressBuf := inst.Invoke.Args.Search("address")
		if addressBuf == nil {
			return nil, nil, errors.New("no address provided")
		}
		address := common.HexToAddress(string(addressBuf))
		_, db, err := getDB(es)
		if err != nil {
			return nil, nil, err
		}
		ret := db.GetBalance(address)
		if ret == big.NewInt(0) {
			log.LLvl1(address.Hex(), "balance", "0")
		}
		log.LLvl1(address.Hex(), "balance", ret.Uint64(), "wei")
		return nil, nil, nil

	case "credit":
		addressBuf := inst.Invoke.Args.Search("address")
		if addressBuf == nil {
			return nil, nil, errors.New("no address provided")
		}
		address := common.HexToAddress(string(addressBuf))
		memdb, db, err := getDB(es)
		if err != nil {
			return nil, nil, err
		}

		amountBuf := inst.Invoke.Args.Search("amount")
		if amountBuf == nil {
			return nil, nil, errors.New("no amount provided")
		}
		amount := new(big.Int).SetBytes(amountBuf)
		db.AddBalance(address, amount)
		log.Lvl1("balance set to", amount, "wei")

		//Commits the general stateDb
		es.RootHash, err = db.Commit(true)
		if err != nil {
			return nil, nil, err
		}

		//Commits the low level trieDB
		err = db.Database().TrieDB().Commit(es.RootHash, true)
		if err != nil {
			return nil, nil, err
		}

		//Saves the general Ethereum State
		es.DbBuf, err = memdb.Dump()
		if err != nil {
			return nil, nil, err
		}

		//Saves the Ethereum structure
		esBuf, err := protobuf.Encode(&es)
		if err != nil {
			return nil, nil, err
		}
		sc = []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
				ContractBvmID, esBuf, darcID),
		}

	case "transaction":
		//Restores Ethereum state from ES struct
		memdb, db, err := getDB(es)
		if err != nil {
			return nil, nil, err
		}
		//Gets Ethereum transaction buffer
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

		//Sends transaction
		transactionReceipt, err := sendTx(&ethTx, db)
		if err != nil {
			log.ErrFatal(err)
			return nil, nil, err
		}

		if transactionReceipt.ContractAddress.Hex() != nilAddress.Hex() {
			log.LLvl1("contract deployed at:", transactionReceipt.ContractAddress.Hex(), "tx status:", transactionReceipt.Status, "gas used:", transactionReceipt.GasUsed, "tx receipt:", transactionReceipt.TxHash.Hex())
		} else {
			log.LLvl1("transaction to", ethTx.To().Hex(), "from", "tx status:", transactionReceipt.Status, "gas used:", transactionReceipt.GasUsed, "tx receipt:", transactionReceipt.TxHash.Hex())
		}

		//Commits the general stateDb
		es.RootHash, err = db.Commit(true)
		if err != nil {
			return nil, nil, err
		}
		//log.LLvl1(db.GetBalance())

		//Commits the low level trieDB
		err = db.Database().TrieDB().Commit(es.RootHash, true)
		if err != nil {
			return nil, nil, err
		}

		//Saves the general Ethereum State
		es.DbBuf, err = memdb.Dump()
		if err != nil {
			return nil, nil, err
		}

		//Encodes the Ethereum structure
		esBuf, err := protobuf.Encode(&es)
		if err != nil {
			return nil, nil, err
		}

		//Saves structure in Byzcoin state
		sc = []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
				ContractBvmID, esBuf, darcID),
		}

	default:
		err = errors.New("Contract can only display, credit and receive transactions")
		return

	}
	return
}

//sendTx is a helper function that applies the signed transaction to a general state
func sendTx(tx *types.Transaction, db *state.StateDB) (*types.Receipt, error) {

	//Gets parameters defined in params
	chainconfig := getChainConfig()
	config := getVMConfig()

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
	receipt, usedGas, err := core.ApplyTransaction(chainconfig, bc, &nilAddress, gp, db, header, tx, ug, config)
	if err != nil {
		log.Error()
		return nil, err
	}
	//logs := db.GetLogs(receipt.TxHash)
	//log.LLvl1("the log we are looking for", db.GetLogs(common.HexToHash("0x47386f9020e5b01eccee6f293ab2b0dc47479f0793e980a4e49bd0c6473e30b1")))
	//log.LLvl1(logs)
	return receipt, nil
}

//EthereumState structure contains DbBuf the general state and RootHash the hash of the last commit
type ES struct {
	DbBuf    []byte
	RootHash common.Hash
}
