package bevm

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// BEvmAttrID identifies the name of the `attr` used in DARC rules
const BEvmAttrID = "bevm"

// ABI template for the verification method. "#METHOD_NAME#" must be replaced
// with the actual method name.
// This corresponds to the signature of a Solidity method as follows:
//
//    struct Argument {
//        string name;
//        bytes value;
//    }
//
//    function myMethod(
//        bytes32 instanceID,
//        string action,
//        Argument[] arguments,
//        int64 protocolVersion,
//        int64 skipBlockIndex,
//        bytes extra
//    ) public view returns (string error) {
//        ...
//    }

const abiTemplate = `` +
	`[` +
	`  {` +
	`    "constant": true,` +
	`    "inputs": [` +
	`      {` +
	`        "name": "instanceID",` +
	`        "type": "bytes32"` +
	`      },` +
	`      {` +
	`        "name": "action",` +
	`        "type": "string"` +
	`      },` +
	`      {` +
	`        "components": [` +
	`          {` +
	`            "name": "name",` +
	`            "type": "string"` +
	`          },` +
	`          {` +
	`            "name": "value",` +
	`            "type": "bytes"` +
	`          }` +
	`        ],` +
	`        "name": "arguments",` +
	`        "type": "tuple[]"` +
	`      },` +
	`      {` +
	`        "name": "skipBlockIndex",` +
	`        "type": "int64"` +
	`      },` +
	`      {` +
	`        "name": "protocolVersion",` +
	`        "type": "int64"` +
	`      },` +
	`      {` +
	`        "name": "extra",` +
	`        "type": "bytes"` +
	`      }` +
	`    ],` +
	`    "name": "#METHOD_NAME#",` +
	`    "outputs": [` +
	`      {` +
	`        "name": "error",` +
	`        "type": "string"` +
	`      }` +
	`    ],` +
	`    "payable": false,` +
	`    "stateMutability": "view",` +
	`    "type": "function"` +
	`  }` +
	`]`

// MakeBevmAttr returns the BEvm `attr` expression evaluator
func MakeBevmAttr(rst byzcoin.ReadOnlyStateTrie,
	inst byzcoin.Instruction, extra []byte) func(string) error {

	return func(attrArgs string) error {
		// Validate arguments
		bevmID, contractAddress, methodName, err := validateArgs(attrArgs)
		if err != nil {
			return fmt.Errorf("failed to validate '"+BEvmAttrID+"' attr "+
				"arguments: %v", err)
		}

		// Retrieve BEvm instance
		value, _, _, _, err := rst.GetValues(bevmID[:])
		if err != nil {
			return fmt.Errorf("failed to retrieve BEvm instance %v: %v",
				bevmID, err)
		}

		// Retrieve BEvm state
		var bs State
		err = protobuf.Decode(value, &bs)
		if err != nil {
			return fmt.Errorf("failed to decode BEvm instance state: %v",
				err)
		}

		// Retrieve EVM stateDB
		byzDb, err := NewStateTrieByzDatabase(bevmID, rst)
		if err != nil {
			return fmt.Errorf("failed to create stateTrie-backed database "+
				"for BEvm: %v", err)
		}

		db := state.NewDatabase(byzDb)
		stateDb, err := state.New(bs.RootHash, db)
		if err != nil {
			return fmt.Errorf("failed to create new EVM db: %v", err)
		}

		// Retrieve the TimeReader (we are actually called with a GlobalState)
		tr, ok := rst.(byzcoin.TimeReader)
		if !ok {
			return fmt.Errorf("internal error: cannot convert " +
				"ReadOnlyStateTrie to TimeReader")
		}

		// Compute the timestamp for the EVM, converting [ns] to [s]
		evmTs := int64(tr.GetCurrentBlockTimestamp() / 1e9)

		// Instantiate a new EVM
		evm := vm.NewEVM(getContext(evmTs), stateDb, getChainConfig(),
			getVMConfig())

		// Pack the call arguments according to ABI
		contractAbi, err := abi.JSON(strings.NewReader(
			strings.ReplaceAll(abiTemplate, "#METHOD_NAME#", methodName)))
		if err != nil {
			return fmt.Errorf("failed to parse ABI for EVM validation view "+
				"method: %v", err)
		}

		// For debug purposes
		log.Lvlf3("instanceID = '%+v'", inst.InstanceID)
		log.Lvlf3("action = '%+v'", inst.Action())
		log.Lvlf3("arguments = '%+v'", inst.Arguments())
		log.Lvlf3("extra = '%+v'", extra)
		log.Lvlf3("skipBlockIndex = '%+v'", rst.GetIndex())
		log.Lvlf3("protocolVersion = '%+v'", rst.GetVersion())

		callData, err := contractAbi.Pack(methodName,
			inst.InstanceID,
			inst.Action(),
			inst.Arguments(),
			int64(rst.GetIndex()),
			int64(rst.GetVersion()),
			extra)
		if err != nil {
			return fmt.Errorf("failed to pack args for EVM validation view "+
				"method: %v", err)
		}

		// Perform the call (maxing the consumption to 1 Ether)
		ret, _, err := evm.Call(vm.AccountRef(nilAddress),
			contractAddress, callData, uint64(1*WeiPerEther),
			big.NewInt(0))
		if err != nil {
			return fmt.Errorf("failed to execute EVM validation view "+
				"method: %v (does the method exist?)", err)
		}

		// Unpack the result
		result, err := unpackResult(contractAbi, methodName, ret)
		if err != nil {
			return fmt.Errorf("failed to unpack EVM validation view "+
				"method result: %v (is the contract address valid?)", err)
		}

		errorMsg, ok := result.(string)
		if !ok {
			return fmt.Errorf("EVM validation view method did not return "+
				"expected type: %+v (%+v)", result,
				reflect.TypeOf(result))
		}

		if errorMsg != "" {
			return fmt.Errorf("'bevm' attribute failed verification: %v",
				errorMsg)
		}

		return nil
	}
}

// 32 bytes in hex
var rxInstanceID = regexp.MustCompile(`^[[:xdigit:]]{64}$`)

// 20 bytes in hex, optionally starting with "0x" or "0X"
var rxEvmAddress = regexp.MustCompile(`^(0[xX])?[[:xdigit:]]{40}$`)

// See 'Identifier' at
// https://github.com/ethereum/solidity/blob/develop/docs/grammar.txt
var rxMethodName = regexp.MustCompile(`^[a-zA-Z_$][a-zA-Z_$0-9]*$`)

func validateArgs(attrArgs string) (bevmID byzcoin.InstanceID,
	contractAddress common.Address, methodName string, err error) {
	const format = "bevm_id:contract_address:method_name"

	args := strings.Split(attrArgs, ":")

	const errPrefix = "failed to parse '" + BEvmAttrID + "' attr arguments: "

	if len(args) != 3 {
		err = fmt.Errorf(errPrefix+
			"expected 3 arguments, got %d (format is %s)",
			len(args), format)
		return
	}

	if !rxInstanceID.MatchString(args[0]) {
		err = fmt.Errorf(errPrefix+
			"argument #1 must be a BEvm instance ID in hex, received '%v'",
			args[0])
		return
	}
	id, err := hex.DecodeString(args[0])
	if err != nil {
		err = fmt.Errorf(errPrefix+
			"argument #1 must be a BEvm instance ID in hex, received '%v' "+
			"(this should have errored before)", args[0])
		return
	}
	bevmID = byzcoin.NewInstanceID(id)

	if !rxEvmAddress.MatchString(args[1]) {
		err = fmt.Errorf(errPrefix+
			"argument #2 must be an EVM contract address in hex, received '%v'",
			args[1])
		return
	}
	contractAddress = common.HexToAddress(args[1])

	if !rxMethodName.MatchString(args[2]) {
		err = fmt.Errorf(errPrefix+
			"argument #3 must be an method name, received '%v'",
			args[2])
		return
	}
	methodName = args[2]

	return
}
