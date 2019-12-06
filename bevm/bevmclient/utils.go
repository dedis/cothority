package main

import (
	"encoding/hex"
	"encoding/json"
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
				return xerrors.Errorf("writing file: %v", err)
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
		return xerrors.Errorf("marshalling account data: %v", err)
	}

	return writeFile(fmt.Sprintf("%s.bevm_account", name), jsonData, check)
}

func readAccountFile(name string) (*bevm.EvmAccount, error) {
	jsonData, err := ioutil.ReadFile(fmt.Sprintf("%s.bevm_account", name))
	if err != nil {
		return nil, xerrors.Errorf("reading account file: %v", err)
	}

	var tmp userEvmAccount
	err = json.Unmarshal(jsonData, &tmp)
	if err != nil {
		return nil, xerrors.Errorf("unmarshalling account data: %v", err)
	}

	account, err := bevm.NewEvmAccount(tmp.PrivateKey)
	if err != nil {
		return nil, xerrors.Errorf("creating new account from file data: %v", err)
	}

	account.Nonce = tmp.Nonce

	return account, nil
}

type userEvmContract struct {
	Abi     string
	Address common.Address
}

func writeContractFile(contractInstance *bevm.EvmContractInstance,
	abiFilepath string, name string, check bool) error {
	jsonAbi, err := ioutil.ReadFile(abiFilepath)
	if err != nil {
		return xerrors.Errorf("reading contract ABI: %v", err)
	}

	tmp := userEvmContract{
		Abi:     string(jsonAbi),
		Address: contractInstance.Address,
	}

	jsonData, err := json.Marshal(tmp)
	if err != nil {
		return xerrors.Errorf("marshalling contract data: %v", err)
	}

	return writeFile(fmt.Sprintf("%s.bevm_contract", name), jsonData, check)
}

func readContractFile(name string) (*bevm.EvmContractInstance, error) {
	jsonData, err := ioutil.ReadFile(fmt.Sprintf("%s.bevm_contract", name))
	if err != nil {
		return nil, xerrors.Errorf("reading contract file: %v", err)
	}

	var tmp userEvmContract
	err = json.Unmarshal(jsonData, &tmp)
	if err != nil {
		return nil, xerrors.Errorf("unmarshalling contract data: %v", err)
	}

	abi, err := abi.JSON(strings.NewReader(tmp.Abi))
	if err != nil {
		return nil, xerrors.Errorf("unmarshalling contract ABI: %v", err)
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
		return nil, xerrors.Errorf("decoding BEvm ID: %v", err)
	}

	cfg, cl, err := lib.LoadConfig(bcFile)
	if err != nil {
		return nil, xerrors.Errorf("loading ByzCoin config: %v", err)
	}

	var signer *darc.Signer
	if signerStr != "" {
		signer, err = lib.LoadKeyFromString(signerStr)
	} else {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	}
	if err != nil {
		return nil, xerrors.Errorf("loading signer key: %v", err)
	}

	bevmClient, err := bevm.NewClient(cl, *signer, byzcoin.NewInstanceID(bevmID))
	if err != nil {
		return nil, xerrors.Errorf("retrieving BEvm client instance: %v", err)
	}

	account, err := readAccountFile(accountName)
	if err != nil {
		return nil, xerrors.Errorf("reading account from file: %v", err)
	}

	return &commonOptions{
		account:     account,
		accountName: accountName,
		bevmClient:  bevmClient,
	}, nil
}

func decodeEvmArgs(encodedArgs []string, abi abi.Arguments) ([]interface{}, error) {
	args := make([]interface{}, len(encodedArgs))
	for i, argJSON := range encodedArgs {
		var arg interface{}
		err := json.Unmarshal([]byte(argJSON), &arg)
		if err != nil {
			return nil, xerrors.Errorf("decoding args for EVM: %v", err)
		}

		switch abi[i].Type.String() {
		case "uint256":
			// The JSON unmarshaller decodes numbers as 'float64'; the EVM expects BigInt
			args[i] = big.NewInt(int64(arg.(float64)))
		case "address":
			args[i] = common.HexToAddress(arg.(string))
		default:
			return nil, xerrors.Errorf("unsupported argument type: %s", abi[i].Type)
		}

		log.Lvlf2("arg #%d: %v (%s) --%v--> %v (%v)",
			i, arg, reflect.TypeOf(arg).Kind(), abi[i].Type, args[i], reflect.TypeOf(args[i]).Kind())
	}

	return args, nil
}
