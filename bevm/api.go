package bevm

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// WeiPerEther represents the number of Wei (the smallest currency denomination
// in Ethereum) in a single Ether.
const WeiPerEther = 1e18

// ---------------------------------------------------------------------------

// EvmContract is the abstraction for an Ethereum contract
type EvmContract struct {
	Abi      abi.ABI
	Bytecode []byte
	name     string // For informational purposes only
}

// EvmContractInstance is a deployed instance of an EvmContract
type EvmContractInstance struct {
	Parent  *EvmContract // The contract of which this is an instance
	Address common.Address
}

// NewEvmContract creates a new EvmContract and stores its ABI and bytecode.
func NewEvmContract(name string, abiJSON string, binData string) (
	*EvmContract, error) {
	contractAbi, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, xerrors.Errorf("failed to decode JSON for "+
			"contract ABI: %v", err)
	}

	contractBytecode := common.Hex2Bytes(binData)

	return &EvmContract{
		name:     name,
		Abi:      contractAbi,
		Bytecode: contractBytecode,
	}, nil
}

func (contract EvmContract) String() string {
	return fmt.Sprintf("EvmContract[%s]", contract.name)
}

func (contract EvmContract) packConstructor(args ...interface{}) (
	[]byte, error) {
	return contract.Abi.Pack("", args...)
}

func (contractInstance EvmContractInstance) String() string {
	return fmt.Sprintf("EvmContractInstance[%s @%s]",
		contractInstance.Parent.name, contractInstance.Address.Hex())
}

func (contractInstance EvmContractInstance) packMethod(method string,
	args ...interface{}) ([]byte, error) {
	return contractInstance.Parent.Abi.Pack(method, args...)
}

func (contractInstance EvmContractInstance) getAbi() abi.ABI {
	return contractInstance.Parent.Abi
}

func unpackResult(contractAbi abi.ABI, methodName string,
	resultBytes []byte) (interface{}, error) {
	methodAbi, ok := contractAbi.Methods[methodName]
	if !ok {
		return nil, xerrors.Errorf("method \"%s\" does not exist for "+
			"this contract", methodName)
	}

	return unpackData(contractAbi, methodName, resultBytes, methodAbi.Outputs)
}

func unpackData(contractAbi abi.ABI, objectName string,
	dataBytes []byte, args abi.Arguments) (interface{}, error) {

	switch len(args) {
	case 0:
		return nil, nil

	case 1:
		// Create a pointer to the desired type
		result := reflect.New(args[0].Type.Type)

		err := contractAbi.Unpack(result.Interface(),
			objectName, dataBytes)
		if err != nil {
			return nil, xerrors.Errorf("failed to unpack single "+
				"element of EVM data: %v", err)
		}

		// Dereference the result pointer
		return result.Elem().Interface(), nil

	default:
		// Abi.Unpack() on multiple values supports a struct or array/slice as
		// structure into which the result is stored. Struct is cleaner, but it
		// does not support unnamed outputs ("or purely underscored"). If this
		// is needed, an array implementation, commented out, follows.

		// Build a struct naming the fields after the outputs
		var fields []reflect.StructField
		for _, output := range args {
			// Adapt names to what Abi.Unpack() does
			name := abi.ToCamelCase(output.Name)

			fields = append(fields, reflect.StructField{
				Name: name,
				Type: output.Type.Type,
			})
		}

		structType := reflect.StructOf(fields)
		s := reflect.New(structType)

		err := contractAbi.Unpack(s.Interface(),
			objectName, dataBytes)
		if err != nil {
			return nil, xerrors.Errorf("failed to unpack multiple "+
				"elements of EVM data: %v", err)
		}

		// Dereference the result pointer
		return s.Elem().Interface(), nil

		// // Build an array of interface{}
		// var empty interface{}
		// arrType := reflect.ArrayOf(len(abiOutputs),
		// 	reflect.ValueOf(&empty).Type().Elem())
		// result := reflect.New(arrType)

		// // Create a value of the desired type for each output
		// for i, output := range abiOutputs {
		// 	val := reflect.New(output.Type.Type)
		// 	result.Elem().Index(i).Set(val)
		// }

		// err := contractInstance.Parent.Abi.Unpack(result.Interface(),
		// 	method, resultBytes)
		// if err != nil {
		// 	return nil, xerrors.Errorf("unpacking multiple result: %v", err)
		// }

		// for i := range abiOutputs {
		// 	val := result.Elem().Index(i)
		// 	// Need to dereference values twice:
		// 	// val is interface{}, *val is *type, **val is type
		// 	val.Set(val.Elem().Elem())
		// }

		// // Dereference the result pointer
		// return result.Elem().Interface(), nil
	}
}

// ---------------------------------------------------------------------------

// EvmAccount is the abstraction for an Ethereum account
type EvmAccount struct {
	Address    common.Address
	PrivateKey *ecdsa.PrivateKey
	Nonce      uint64
}

// NewEvmAccount creates a new EvmAccount
func NewEvmAccount(privateKey string) (*EvmAccount, error) {
	privKey, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, xerrors.Errorf("failed to decode private "+
			"key for account creation: %v", err)
	}

	address := crypto.PubkeyToAddress(privKey.PublicKey)

	return &EvmAccount{
		Address:    address,
		PrivateKey: privKey,
	}, nil
}

func (account EvmAccount) String() string {
	return fmt.Sprintf("EvmAccount[%s]", account.Address.Hex())
}

// ---------------------------------------------------------------------------

// Client is the abstraction for the ByzCoin EVM client
type Client struct {
	*onet.Client
	bcClient   *byzcoin.Client
	signer     darc.Signer
	instanceID byzcoin.InstanceID
}

// NewBEvm creates a new ByzCoin EVM instance
func NewBEvm(bcClient *byzcoin.Client, signer darc.Signer, gDarc *darc.Darc) (
	byzcoin.InstanceID, error) {
	instanceID := byzcoin.NewInstanceID(nil)

	tx, err := spawnBEvm(bcClient, signer,
		byzcoin.NewInstanceID(gDarc.GetBaseID()), &byzcoin.Spawn{
			ContractID: ContractBEvmID,
			Args:       byzcoin.Arguments{},
		})
	if err != nil {
		return instanceID, xerrors.Errorf("failed to execute ByzCoin "+
			"spawn command for BEvm contract: %v", err)
	}

	instanceID = tx.Instructions[0].DeriveID("")

	return instanceID, nil
}

// NewClient creates a new ByzCoin EVM client, connected to the given ByzCoin
// instance
func NewClient(bcClient *byzcoin.Client, signer darc.Signer,
	instanceID byzcoin.InstanceID) (*Client, error) {
	return &Client{
		Client:     onet.NewClient(cothority.Suite, ServiceName),
		bcClient:   bcClient,
		signer:     signer,
		instanceID: instanceID,
	}, nil
}

// Delete deletes the ByzCoin EVM client and all its state
func (client *Client) Delete() error {
	_, err := client.deleteBEvm(&byzcoin.Delete{
		ContractID: ContractBEvmID,
	})
	if err != nil {
		return xerrors.Errorf("failed to execute ByzCoin "+
			"delete command for BEvm instance: %v", err)
	}

	return nil
}

// Deploy deploys a new Ethereum contract on the EVM
func (client *Client) Deploy(gasLimit uint64, gasPrice uint64, amount uint64,
	account *EvmAccount, contract *EvmContract, args ...interface{}) (
	*byzcoin.ClientTransaction, *EvmContractInstance, error) {
	log.Lvlf2(">>> Deploy EVM contract '%s'", contract)
	defer log.Lvlf2("<<< Deploy EVM contract '%s'", contract)

	packedArgs, err := contract.packConstructor(args...)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to pack arguments for "+
			"contract constructor: %v", err)
	}

	callData := append(contract.Bytecode, packedArgs...)
	tx := types.NewContractCreation(account.Nonce, big.NewInt(int64(amount)),
		gasLimit, big.NewInt(int64(gasPrice)), callData)
	signedTxBuffer, err := account.signAndMarshalTx(tx)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to prepare EVM transaction "+
			"for EVM contract deployment: %v", err)
	}

	bcTx, err := client.invoke("transaction", byzcoin.Arguments{
		{Name: "tx", Value: signedTxBuffer},
	})
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to invoke ByzCoin transaction "+
			"for EVM contract deployment: %v", err)
	}

	contractInstance := &EvmContractInstance{
		Parent:  contract,
		Address: crypto.CreateAddress(account.Address, account.Nonce),
	}

	account.Nonce++

	return bcTx, contractInstance, nil
}

// Transaction performs a new transaction (contract method call with state
// change) on the EVM
func (client *Client) Transaction(gasLimit uint64, gasPrice uint64,
	amount uint64, account *EvmAccount, contractInstance *EvmContractInstance,
	method string, args ...interface{}) (*byzcoin.ClientTransaction, error) {
	log.Lvlf2(">>> EVM method '%s()' on %s", method, contractInstance)
	defer log.Lvlf2("<<< EVM method '%s()' on %s", method, contractInstance)

	callData, err := contractInstance.packMethod(method, args...)
	if err != nil {
		return nil, xerrors.Errorf("failed to pack arguments for contract "+
			"method '%s': %v", method, err)
	}

	tx := types.NewTransaction(account.Nonce, contractInstance.Address,
		big.NewInt(int64(amount)), gasLimit, big.NewInt(int64(gasPrice)),
		callData)
	signedTxBuffer, err := account.signAndMarshalTx(tx)
	if err != nil {
		return nil, xerrors.Errorf("failed to prepare EVM transaction for "+
			"EVM method execution: %v", err)
	}

	bcTx, err := client.invoke("transaction", byzcoin.Arguments{
		{Name: "tx", Value: signedTxBuffer},
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to invoke ByzCoin transaction for "+
			"EVM method execution: %v", err)
	}

	account.Nonce++

	return bcTx, nil
}

// Call performs a new call (contract view method call, without state change)
// on the EVM
func (client *Client) Call(account *EvmAccount,
	contractInstance *EvmContractInstance,
	method string, args ...interface{}) (interface{}, error) {
	log.Lvlf2(">>> EVM view method '%s()' on %s", method, contractInstance)
	defer log.Lvlf2("<<< EVM view method '%s()' on %s",
		method, contractInstance)

	// Pack the method call and arguments
	callData, err := contractInstance.packMethod(method, args...)
	if err != nil {
		return nil, xerrors.Errorf("failed to pack arguments for contract "+
			"view method '%s': %v", method, err)
	}

	// Retrieve the EVM state
	stateDb, err := getEvmDb(client.bcClient, client.instanceID)
	if err != nil {
		return nil, xerrors.Errorf("failed to retrieve EVM state: %v", err)
	}

	// Compute timestamp for the EVM
	timestamp := time.Now().UnixNano()
	// timestamp in ByzCoin is in [ns], whereas in EVM it is in [s]
	evmTs := timestamp / 1e9

	ret, err := CallEVM(account.Address, contractInstance.Address, callData,
		stateDb, evmTs)
	if err != nil {
		return nil, xerrors.Errorf("failed to call EVM: %v", err)
	}

	// Unpack the result
	result, err := unpackResult(contractInstance.getAbi(), method, ret)
	if err != nil {
		return nil, xerrors.Errorf("failed to unpack EVM view "+
			"method result: %v", err)
	}

	return result, nil
}

// CallEVM performs a low-level call (contract view method call, without state
// change) on the EVM, using ABI packed data, ethereum addresses, a stateDB and
// a timestamp representing `now`.
func CallEVM(accountAddress common.Address, contractAddress common.Address,
	callData []byte, stateDb *state.StateDB, ts int64) ([]byte, error) {
	log.Lvlf2(">>> Call EVM %v → %v [%v]", accountAddress.Hex(),
		contractAddress.Hex(), hex.EncodeToString(callData))
	defer log.Lvlf2("<<< Call EVM %v → %v [%v]", accountAddress.Hex(),
		contractAddress.Hex(), hex.EncodeToString(callData))

	// Instantiate a new EVM
	evm := vm.NewEVM(getContext(ts), stateDb, getChainConfig(),
		getVMConfig())

	// Perform the call (1 Ether should be enough for everyone [tm]...)
	ret, _, err := evm.Call(vm.AccountRef(accountAddress), contractAddress,
		callData, uint64(1*WeiPerEther), big.NewInt(0))
	if err != nil {
		return nil, xerrors.Errorf("failed to execute EVM call: %v ", err)
	}

	return ret, nil
}

// CreditAccount credits the given Ethereum address with the given amount
func (client *Client) CreditAccount(amount *big.Int,
	address common.Address) (*byzcoin.ClientTransaction, error) {
	bcTx, err := client.invoke("credit", byzcoin.Arguments{
		{Name: "address", Value: address.Bytes()},
		{Name: "amount", Value: amount.Bytes()},
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to credit EVM account: %v", err)
	}

	log.Lvlf2("Credited %d wei on '%x'", amount, address)

	return bcTx, nil
}

// GetAccountBalance returns the current balance of a Ethereum address
func (client *Client) GetAccountBalance(address common.Address) (
	*big.Int, error) {
	stateDb, err := getEvmDb(client.bcClient, client.instanceID)
	if err != nil {
		return nil, xerrors.Errorf("failed to retrieve EVM state: %v", err)
	}

	balance := stateDb.GetBalance(address)

	log.Lvlf2("Balance of '%x' is %d wei", address, balance)

	return balance, nil
}

// ---------------------------------------------------------------------------
// Service methods

// PerformCall sends a request to execute a Call (R-only method, "view method")
// on a previously deployed EVM contract instance. Returns the call response.
func (client *Client) PerformCall(dst *network.ServerIdentity, byzcoinID []byte,
	bevmInstanceID byzcoin.InstanceID, accountAddress []byte,
	contractAddress []byte, callData []byte) (*CallResponse, error) {
	request := &CallRequest{
		ByzCoinID:       byzcoinID,
		BEvmInstanceID:  bevmInstanceID[:],
		AccountAddress:  accountAddress,
		ContractAddress: contractAddress,
		CallData:        callData,
	}
	response := &CallResponse{}

	err := client.Client.SendProtobuf(dst, request, response)
	if err != nil {
		return nil, err
	}

	return response, err
}

// ---------------------------------------------------------------------------
// Utility functions

// DecodeEvmArgs decodes a list of arguments encoded in JSON into Go values,
// suitable to be used as arguments to contract transactions and calls.
// This can be useful for command-line tools or calls serialized over the
// network.
func DecodeEvmArgs(encodedArgs []string, abi abi.Arguments) (
	[]interface{}, error) {
	args := make([]interface{}, len(encodedArgs))

	for i, argJSON := range encodedArgs {
		var arg interface{}
		err := json.Unmarshal([]byte(argJSON), &arg)
		if err != nil {
			return nil, xerrors.Errorf("failed to decode JSON-encoded "+
				"arguments for EVM call: %v", err)
		}

		abiTypeStr := abi[i].Type.String()
		arrayBracket := strings.IndexRune(abiTypeStr, '[')
		if arrayBracket == -1 {
			args[i], err = decodeEvmValue(abiTypeStr, abi[i].Type.Type, arg)
			if err != nil {
				return nil, xerrors.Errorf("failed to decode simple-type "+
					"value for EVM call: %v", err)
			}
		} else {
			args[i], err = decodeEvmArray(abiTypeStr[:arrayBracket], abi[i], arg)
			if err != nil {
				return nil, xerrors.Errorf("failed to decode array-type "+
					"value for EVM call: %v", err)
			}
		}

		log.Lvlf2("arg #%d: %v (%s) --%v--> %v (%v)",
			i, arg, reflect.TypeOf(arg).Kind(), abi[i].Type, args[i],
			reflect.TypeOf(args[i]).Kind())
	}

	return args, nil
}

func decodeEvmValue(abiType string, argType reflect.Type, arg interface{}) (
	interface{}, error) {
	var decodedArg interface{}

	switch abiType {
	case "uint", "uint256", "uint128", "int", "int256", "int128":
		// We use strings to ensure big numbers are handled properly in JSON
		argAsString, ok := arg.(string)
		if !ok {
			return nil, xerrors.Errorf("received '%v' for value of type "+
				"'%s', expected to be a JSON string", arg, abiType)
		}

		val, ok := big.NewInt(0).SetString(argAsString, 0)
		if !ok {
			return nil, xerrors.Errorf("invalid big number value: %v", arg)
		}

		decodedArg = val

	case "uint32", "uint16", "uint8":
		// The JSON unmarshaller decodes numbers as 'float64', while the EVM
		// expects uint{32,16,8}
		argAsFloat, ok := arg.(float64)
		if !ok {
			return nil, xerrors.Errorf("received '%v' for value of type "+
				"'%s', expected to be a JSON number", arg, abiType)
		}

		v := reflect.New(argType).Elem()
		v.SetUint(uint64(argAsFloat))

		decodedArg = v.Interface()

	case "int32", "int16", "int8":
		// The JSON unmarshaller decodes numbers as 'float64', while the EVM
		// expects int{32,16,8}
		argAsFloat, ok := arg.(float64)
		if !ok {
			return nil, xerrors.Errorf("received '%v' for value of type "+
				"'%s', expected to be a JSON number", arg, abiType)
		}

		v := reflect.New(argType).Elem()
		v.SetInt(int64(argAsFloat))

		decodedArg = v.Interface()

	case "address":
		argAsString, ok := arg.(string)
		if !ok {
			return nil, xerrors.Errorf("received '%v' for value of type "+
				"'%s', expected to be a JSON string", arg, abiType)
		}

		decodedArg = common.HexToAddress(argAsString)

	case "string":
		argAsString, ok := arg.(string)
		if !ok {
			return nil, xerrors.Errorf("received '%v' for value of type "+
				"'%s', expected to be a JSON string", arg, abiType)
		}

		decodedArg = argAsString

	default:
		return nil, xerrors.Errorf("unsupported type for EVM argument "+
			"value: %s", abiType)
	}

	return decodedArg, nil
}

func decodeEvmArray(abiType string, abi abi.Argument, arg interface{}) (
	interface{}, error) {
	// Create a pointer to the desired array type and dereference it
	arr := reflect.New(abi.Type.Type).Elem()

	argAsArray, ok := arg.([]interface{})
	if !ok {
		return nil, xerrors.Errorf("received '%v' for value of type "+
			"'%s[%d]', expected to be a JSON array", arg, abiType, arr.Len())
	}
	if len(argAsArray) != arr.Len() {
		return nil, xerrors.Errorf("incorrect array size %d for value of "+
			"type '%s[%d]'", len(argAsArray), abiType, arr.Len())
	}

	// Decode the elements and fill the array
	for i := 0; i < arr.Len(); i++ {
		val, err := decodeEvmValue(abiType, abi.Type.Type.Elem(), argAsArray[i])
		if err != nil {
			return nil, xerrors.Errorf("failed to decode element of "+
				"EVM array value: %v", err)
		}
		arr.Index(i).Set(reflect.ValueOf(val))
	}

	return arr.Interface(), nil
}

// EncodeEvmReturnValue encodes the return value of an EVM call to JSON.
// This can be useful for command-line tools or calls serialized over the
// network.
func EncodeEvmReturnValue(returnValue interface{},
	outputs abi.Arguments) (string, error) {
	var jsonReturnValue []byte
	var err error

	if len(outputs) == 1 {
		abiType := outputs[0].Type.String()
		encodedReturnValue, err := encodeEvmValue(abiType, returnValue)
		if err != nil {
			return "", xerrors.Errorf("failed to encode EVM value: %v", err)
		}

		jsonReturnValue, err = json.Marshal(encodedReturnValue)
	} else {
		returnValueSlice, ok := returnValue.([]interface{})
		if !ok {
			return "", xerrors.Errorf("received return value of type: %v, "+
				"expected to be a slice", reflect.TypeOf(returnValue))
		}
		if len(returnValueSlice) != len(outputs) {
			return "", xerrors.Errorf("received slice of length %v, expected "+
				"to be of length %v", len(returnValueSlice), len(outputs))
		}

		encodedReturnValue := make([]interface{}, len(outputs))
		for i, output := range outputs {
			abiType := output.Type.String()
			encodedReturnValue[i], err = encodeEvmValue(abiType,
				returnValueSlice[i])
			if err != nil {
				return "", xerrors.Errorf("failed to encode EVM value: %v", err)
			}
		}

		jsonReturnValue, err = json.Marshal(encodedReturnValue)
	}

	if err != nil {
		return "", xerrors.Errorf("failed to encode return value to JSON: %v",
			err)
	}

	return string(jsonReturnValue), nil
}

func encodeEvmValue(abiType string, value interface{}) (interface{}, error) {
	var encodedValue interface{}

	switch abiType {
	case "uint", "uint256", "uint128", "int", "int256", "int128":
		// We use strings to ensure big numbers are handled properly in JSON
		valueAsBigInt, ok := value.(*big.Int)
		if !ok {
			return nil, xerrors.Errorf("received '%v' for return value of "+
				"type '%s', expected to be a *big.Int", value, abiType)
		}

		encodedValue = valueAsBigInt.String()

	default:
		encodedValue = value
	}

	return encodedValue, nil
}

// ---------------------------------------------------------------------------
// Helper functions

// signAndMarshalTx signs an Ethereum transaction and returns it in byte
// format, ready to be included into a Byzcoin transaction
func (account EvmAccount) signAndMarshalTx(tx *types.Transaction) (
	[]byte, error) {
	var signer types.Signer = types.HomesteadSigner{}

	signedTx, err := types.SignTx(tx, signer, account.PrivateKey)
	if err != nil {
		return nil, xerrors.Errorf("failed to sign EVM transaction: %v", err)
	}

	signedBuffer, err := signedTx.MarshalJSON()
	if err != nil {
		return nil, xerrors.Errorf("failed to serialize EVM transaction "+
			"to JSON: %v", err)
	}

	return signedBuffer, nil
}

// Retrieve a read-only EVM state database backed by a ByzCoin client
func getEvmDb(bcClient *byzcoin.Client, instID byzcoin.InstanceID) (
	*state.StateDB, error) {
	// Retrieve the proof of the Byzcoin instance
	proofResponse, err := bcClient.GetProofFromLatest(instID[:])
	if err != nil {
		return nil, xerrors.Errorf("failed to retrieve BEvm instance: %v", err)
	}

	// Extract the value from the proof
	_, value, _, _, err := proofResponse.Proof.KeyValue()
	if err != nil {
		return nil, xerrors.Errorf("failed to get BEvm instance value: %v", err)
	}

	// Decode the proof value into an EVM State
	var bs State
	err = protobuf.Decode(value, &bs)
	if err != nil {
		return nil, xerrors.Errorf("failed to decode BEvm instance "+
			"value: %v", err)
	}

	// Create a client ByzDB instance
	byzDb, err := NewClientByzDatabase(instID, bcClient)
	if err != nil {
		return nil, xerrors.Errorf("failed to creatw a new ByzDB "+
			"instance: %v", err)
	}

	db := state.NewDatabase(byzDb)

	stateDb, err := state.New(bs.RootHash, db)
	if err != nil {
		return nil, xerrors.Errorf("failed to create new EVM db: %v", err)
	}

	return stateDb, nil
}

// Retrieve a read-only EVM state database backed by a StateTrie
func getEvmDbRst(rst byzcoin.ReadOnlyStateTrie, bevmID byzcoin.InstanceID) (
	*state.StateDB, error) {
	// Retrieve BEvm instance
	value, _, _, _, err := rst.GetValues(bevmID[:])
	if err != nil {
		return nil, xerrors.Errorf("failed to retrieve BEvm instance %v: %v",
			bevmID, err)
	}

	// Retrieve BEvm state
	var bs State
	err = protobuf.Decode(value, &bs)
	if err != nil {
		return nil, xerrors.Errorf("failed to decode BEvm instance state: %v",
			err)
	}

	// Retrieve EVM stateDB
	byzDb, err := NewStateTrieByzDatabase(bevmID, rst)
	if err != nil {
		return nil, xerrors.Errorf("failed to create stateTrie-backed database "+
			"for BEvm: %v", err)
	}

	db := state.NewDatabase(byzDb)
	stateDb, err := state.New(bs.RootHash, db)
	if err != nil {
		return nil, xerrors.Errorf("failed to create new EVM db: %v", err)
	}

	return stateDb, nil
}

// Invoke a method on a ByzCoin EVM instance
func (client *Client) invoke(command string, args byzcoin.Arguments) (
	*byzcoin.ClientTransaction, error) {
	bcTx, err := client.invokeBEvm(&byzcoin.Invoke{
		ContractID: ContractBEvmID,
		Command:    command,
		Args:       args,
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to execute ByzCoin invoke "+
			"instruction: %v", err)
	}

	return bcTx, nil
}

func spawnBEvm(bcClient *byzcoin.Client, signer darc.Signer,
	instanceID byzcoin.InstanceID, instr *byzcoin.Spawn) (
	*byzcoin.ClientTransaction, error) {
	return execByzCoinTx(bcClient, signer, instanceID, instr, nil, nil)
}

func (client *Client) invokeBEvm(instr *byzcoin.Invoke) (
	*byzcoin.ClientTransaction, error) {
	return execByzCoinTx(client.bcClient, client.signer, client.instanceID,
		nil, instr, nil)
}

func (client *Client) deleteBEvm(instr *byzcoin.Delete) (
	*byzcoin.ClientTransaction, error) {
	return execByzCoinTx(client.bcClient, client.signer, client.instanceID,
		nil, nil, instr)
}

func execByzCoinTx(bcClient *byzcoin.Client,
	signer darc.Signer, instanceID byzcoin.InstanceID,
	spawnInstr *byzcoin.Spawn, invokeInstr *byzcoin.Invoke,
	deleteInstr *byzcoin.Delete) (*byzcoin.ClientTransaction, error) {
	counters, err := bcClient.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return nil, xerrors.Errorf("failed to retrieve signer "+
			"counters from ByzCoin: %v", err)
	}

	tx, err := bcClient.CreateTransaction(byzcoin.Instruction{
		InstanceID:    instanceID,
		SignerCounter: []uint64{counters.Counters[0] + 1},
		Spawn:         spawnInstr,
		Invoke:        invokeInstr,
		Delete:        deleteInstr,
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to create ByzCoin "+
			"transaction: %v", err)
	}

	err = tx.FillSignersAndSignWith(signer)
	if err != nil {
		return nil, xerrors.Errorf("failed to sign ByzCoin "+
			"transaction: %v", err)
	}

	// Sending this transaction to ByzCoin does not directly include it in the
	// global state - first we must wait for the new block to be created.
	_, err = bcClient.AddTransactionAndWait(tx, 5)
	if err != nil {
		return nil, xerrors.Errorf("failed to send ByzCoin "+
			"transaction: %v", err)
	}

	return &tx, nil
}
