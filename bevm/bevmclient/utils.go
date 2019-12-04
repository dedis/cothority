package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"reflect"
	"strings"
	"time"

	"go.dedis.ch/cothority/v3/bevm"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	cli "github.com/urfave/cli"
)

type userEvmAccount struct {
	PrivateKey string
	Nonce      uint64
}

func writeFile(file string, data []byte, check bool) error {
	if check {
		fileInfo, err := os.Stat(file)
		if !os.IsNotExist(err) {
			newName := file + "." + fileInfo.ModTime().Format(time.RFC3339)
			fmt.Fprintf(os.Stderr, "WARNING: Previous file exists and is being renamed to '%s'\n", newName)

			err = os.Rename(file, newName)
			if err != nil {
				return err
			}
		}
	}

	return ioutil.WriteFile(file, data, 0600)
}

func writeAccountFile(account *bevm.EvmAccount, name string, check bool) error {
	tmp := userEvmAccount{
		PrivateKey: hex.EncodeToString(crypto.FromECDSA(account.PrivateKey)),
		Nonce:      account.Nonce,
	}

	jsonData, err := json.Marshal(tmp)
	if err != nil {
		return err
	}

	return writeFile(fmt.Sprintf("%s.bevm_account", name), jsonData, check)
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

func writeContractFile(contractInstance *bevm.EvmContractInstance, abiFilepath string, name string, check bool) error {
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

	return writeFile(fmt.Sprintf("%s.bevm_contract", name), jsonData, check)
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

type commonOptions struct {
	account     *bevm.EvmAccount
	accountName string
	bevmClient  *bevm.Client
}

func handleCommonOptions(ctx *cli.Context) (*commonOptions, error) {
	bcFile := ctx.String("bc")
	bevmIDStr := ctx.String("bevm-id")
	signerStr := ctx.String("sign")
	accountName := ctx.String("account-name")

	bevmID, err := hex.DecodeString(bevmIDStr)
	if err != nil {
		return nil, err
	}

	cfg, cl, err := lib.LoadConfig(bcFile)
	if err != nil {
		return nil, err
	}

	var signer *darc.Signer
	if signerStr != "" {
		signer, err = lib.LoadKeyFromString(signerStr)
	} else {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	}
	if err != nil {
		return nil, err
	}

	bevmClient, err := bevm.NewClient(cl, *signer, byzcoin.NewInstanceID(bevmID))
	if err != nil {
		return nil, err
	}

	account, err := readAccountFile(accountName)
	if err != nil {
		return nil, err
	}

	return &commonOptions{
		account:     account,
		accountName: accountName,
		bevmClient:  bevmClient,
	}, nil
}

func getBevmClient(configFile string, signer *darc.Signer, bevmID byzcoin.InstanceID) (*bevm.Client, error) {
	cfg, cl, err := lib.LoadConfig(configFile)
	if err != nil {
		return nil, err
	}

	if signer == nil {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
		if err != nil {
			return nil, err
		}
	}

	return bevm.NewClient(cl, *signer, bevmID)
}

func decodeEvmArgs(encodedArgs []string, abi abi.Arguments) ([]interface{}, error) {
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
