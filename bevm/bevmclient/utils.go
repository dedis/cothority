package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"reflect"
	"strings"

	"go.dedis.ch/cothority/v3/bevm"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type userEvmAccount struct {
	PrivateKey string
	Nonce      uint64
}

func writeAccountFile(account *bevm.EvmAccount, name string) error {
	tmp := userEvmAccount{
		PrivateKey: hex.EncodeToString(crypto.FromECDSA(account.PrivateKey)),
		Nonce:      account.Nonce,
	}

	jsonData, err := json.Marshal(tmp)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(fmt.Sprintf("%s.bevm_account", name), jsonData, 0600)
}

func readAccountFile(name string) (*bevm.EvmAccount, error) {
	jsonData, err := ioutil.ReadFile(fmt.Sprintf("%s.bevm_account", name))
	if err != nil {
		return nil, err
	}

	var tmp userEvmAccount
	err = json.Unmarshal(jsonData, &tmp)
	if err != nil {
		return nil, err
	}

	account, err := bevm.NewEvmAccount(tmp.PrivateKey)
	if err != nil {
		return nil, err
	}

	account.Nonce = tmp.Nonce

	return account, nil
}

type userEvmContract struct {
	Abi     string
	Address common.Address
}

func writeContractFile(contractInstance *bevm.EvmContractInstance, abiFilepath string, name string) error {
	jsonAbi, err := ioutil.ReadFile(abiFilepath)
	if err != nil {
		return errors.New("error reading contract ABI: " + err.Error())
	}

	tmp := userEvmContract{
		Abi:     string(jsonAbi),
		Address: contractInstance.Address,
	}

	jsonData, err := json.Marshal(tmp)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(fmt.Sprintf("%s.bevm_contract", name), jsonData, 0600)
}

func readContractFile(name string) (*bevm.EvmContractInstance, error) {
	jsonData, err := ioutil.ReadFile(fmt.Sprintf("%s.bevm_contract", name))
	if err != nil {
		return nil, err
	}

	var tmp userEvmContract
	err = json.Unmarshal(jsonData, &tmp)
	if err != nil {
		return nil, xerrors.Errorf("unmarshalling JSON data: %w", err)
	}

	abi, err := abi.JSON(strings.NewReader(tmp.Abi))
	if err != nil {
		return nil, xerrors.Errorf("unmarshalling JSON ABI: %w", err)
	}

	contractInstance := bevm.EvmContractInstance{
		Parent: &bevm.EvmContract{
			Abi: abi,
		},
		Address: tmp.Address,
	}

	return &contractInstance, nil
}

func getBevmClient(file string, bevmID []byte) (*bevm.Client, error) {
	cfg, cl, err := lib.LoadConfig(file)
	if err != nil {
		return nil, err
	}

	signer, err := lib.LoadKey(cfg.AdminIdentity)
	if err != nil {
		return nil, err
	}

	return bevm.NewClient(cl, *signer, byzcoin.NewInstanceID(bevmID))
}

func decodeArgs(encodedArgs []string, abi abi.Arguments) ([]interface{}, error) {
	args := make([]interface{}, len(encodedArgs))
	for i, argJSON := range encodedArgs {
		var arg interface{}
		err := json.Unmarshal([]byte(argJSON), &arg)
		if err != nil {
			return nil, err
		}

		switch abi[i].Type.String() {
		case "uint256":
			// The JSON unmarshaller decodes numbers as 'float64'; the EVM expects BigInt
			args[i] = big.NewInt(int64(arg.(float64)))
		case "address":
			args[i] = common.HexToAddress(arg.(string))
		default:
			return nil, fmt.Errorf("Unsupported argument type: %s", abi[i].Type)
		}

		log.Lvlf2("arg #%d: %v (%s) --%v--> %v (%v)",
			i, arg, reflect.TypeOf(arg).Kind(), abi[i].Type, args[i], reflect.TypeOf(args[i]).Kind())
	}

	return args, nil
}
