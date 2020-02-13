package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"go.dedis.ch/cothority/v3/bevm"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/darc"
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
		if err == nil {
			// No error --> the file exists
			newName := file + "." + fileInfo.ModTime().Format(time.RFC3339)
			fmt.Fprintf(os.Stderr, "WARNING: Previous file exists "+
				"and is being renamed to '%s'\n", newName)

			err = os.Rename(file, newName)
			if err != nil {
				return xerrors.Errorf("failed to rename file: %v", err)
			}
		} else if !os.IsNotExist(err) {
			return xerrors.Errorf("failed to stat file: %v", err)
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
		return xerrors.Errorf("failed to serialize account "+
			"information: %v", err)
	}

	return writeFile(fmt.Sprintf("%s.bevm_account", name), jsonData, check)
}

func readAccountFile(name string) (*bevm.EvmAccount, error) {
	jsonData, err := ioutil.ReadFile(fmt.Sprintf("%s.bevm_account", name))
	if err != nil {
		return nil, xerrors.Errorf("failed to read account file: %v", err)
	}

	var tmp userEvmAccount
	err = json.Unmarshal(jsonData, &tmp)
	if err != nil {
		return nil, xerrors.Errorf("failed to deserialize account "+
			"information: %v", err)
	}

	account, err := bevm.NewEvmAccount(tmp.PrivateKey)
	if err != nil {
		return nil, xerrors.Errorf("failed to create new account from "+
			"file data: %v", err)
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
		return xerrors.Errorf("failed to read contract ABI: %v", err)
	}

	tmp := userEvmContract{
		Abi:     string(jsonAbi),
		Address: contractInstance.Address,
	}

	jsonData, err := json.Marshal(tmp)
	if err != nil {
		return xerrors.Errorf("failed to serialize contract "+
			"information: %v", err)
	}

	return writeFile(fmt.Sprintf("%s.bevm_contract", name), jsonData, check)
}

func readContractFile(name string) (*bevm.EvmContractInstance, error) {
	jsonData, err := ioutil.ReadFile(fmt.Sprintf("%s.bevm_contract", name))
	if err != nil {
		return nil, xerrors.Errorf("failed to read contract file: %v", err)
	}

	var tmp userEvmContract
	err = json.Unmarshal(jsonData, &tmp)
	if err != nil {
		return nil, xerrors.Errorf("failed to deserialize contract "+
			"information: %v", err)
	}

	abi, err := abi.JSON(strings.NewReader(tmp.Abi))
	if err != nil {
		return nil, xerrors.Errorf("failed to deserialize contract "+
			"ABI: %v", err)
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
	bevmIDStr := ctx.String("bevmID")
	signerStr := ctx.String("sign")
	accountName := ctx.String("accountName")

	bevmID, err := hex.DecodeString(bevmIDStr)
	if err != nil {
		return nil, xerrors.Errorf("failed to decode BEvm ID: %v", err)
	}

	cfg, cl, err := lib.LoadConfig(bcFile)
	if err != nil {
		return nil, xerrors.Errorf("failed to load ByzCoin config: %v", err)
	}

	var signer *darc.Signer
	if signerStr != "" {
		signer, err = lib.LoadKeyFromString(signerStr)
	} else {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	}
	if err != nil {
		return nil, xerrors.Errorf("failed to load signer key: %v", err)
	}

	bevmClient, err := bevm.NewClient(cl, *signer,
		byzcoin.NewInstanceID(bevmID))
	if err != nil {
		return nil, xerrors.Errorf("failed to retrieve BEvm client "+
			"instance: %v", err)
	}

	account, err := readAccountFile(accountName)
	if err != nil {
		return nil, xerrors.Errorf("failed to read account from file: %v", err)
	}

	return &commonOptions{
		account:     account,
		accountName: accountName,
		bevmClient:  bevmClient,
	}, nil
}
