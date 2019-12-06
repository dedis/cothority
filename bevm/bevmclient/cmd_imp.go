package main

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"reflect"
	"strconv"

	"golang.org/x/xerrors"

	cli "github.com/urfave/cli"
	"go.dedis.ch/cothority/v3/bevm"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func createAccount(ctx *cli.Context) error {
	// Retrieve arguments

	name := ctx.String("account-name")

	// Perform command

	pk, err := crypto.GenerateKey()
	if err != nil {
		return xerrors.Errorf("generating EVM account key: %v", err)
	}

	account, err := bevm.NewEvmAccount(common.Bytes2Hex(crypto.FromECDSA(pk)))
	if err != nil {
		return xerrors.Errorf("creating new EVM account: %v", err)
	}

	fmt.Printf("New account \"%s\" created at address: %s\n", name, account.Address.String())

	return writeAccountFile(account, name, true)
}

func creditAccount(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return xerrors.Errorf("handling options: %v", err)
	}

	if !ctx.Args().Present() {
		return xerrors.New("missing amount argument")
	}

	amountStr := ctx.Args().First()
	amount, err := strconv.ParseUint(amountStr, 0, 64)
	if err != nil {
		return xerrors.Errorf("parsing amount value: %v", err)
	}

	// Perform command

	amountBig := big.NewInt(int64(amount))
	amountBig.Mul(amountBig, big.NewInt(bevm.WeiPerEther))

	err = opt.bevmClient.CreditAccount(amountBig, opt.account.Address)
	if err != nil {
		return xerrors.Errorf("crediting BEvm account: %v", err)
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "Credited account %s with %d Ether\n",
		opt.account.Address.Hex(), amount)
	if err != nil {
		return xerrors.Errorf("writing report msg: %v", err)
	}

	return nil
}

func getAccountBalance(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return xerrors.Errorf("handling options: %v", err)
	}

	// Perform command

	amount, err := opt.bevmClient.GetAccountBalance(opt.account.Address)
	if err != nil {
		return xerrors.Errorf("retrieving BEvm account balance: %v", err)
	}

	var amountEther, amountWei big.Int
	amountEther.DivMod(amount, big.NewInt(bevm.WeiPerEther), &amountWei)
	_, err = fmt.Fprintf(ctx.App.Writer, "Balance of account %s: %v Ether, %v Wei\n",
		opt.account.Address.Hex(), amountEther.String(), amountWei.String())
	if err != nil {
		return xerrors.Errorf("writing report msg: %v", err)
	}

	return nil
}

func deployContract(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return xerrors.Errorf("handling options: %v", err)
	}

	gasLimit := ctx.Uint64("gas-limit")
	gasPrice := ctx.Uint64("gas-price")
	amount := ctx.Uint64("amount")
	contractName := ctx.String("contract-name")

	if ctx.NArg() < 2 {
		return xerrors.Errorf("missing some argument (expected 2, got %d)", ctx.NArg())
	}

	abiFilepath := ctx.Args().Get(0)
	binFilepath := ctx.Args().Get(1)

	abiData, err := ioutil.ReadFile(abiFilepath)
	if err != nil {
		return xerrors.Errorf("reading contract ABI: %v", err)
	}

	binData, err := ioutil.ReadFile(binFilepath)
	if err != nil {
		return xerrors.Errorf("reading contract Bytecode: %v", err)
	}

	contract, err := bevm.NewEvmContract("newContract", string(abiData), string(binData))
	if err != nil {
		return xerrors.Errorf("creating new BEvm contract: %v", err)
	}

	userArgs := ctx.Args()[2:]
	args, err := decodeEvmArgs(userArgs, contract.Abi.Constructor.Inputs)
	if err != nil {
		return xerrors.Errorf("decoding contract constructor arguments: %v", err)
	}

	// Perform command

	contractInstance, err := opt.bevmClient.Deploy(
		gasLimit, big.NewInt(int64(gasPrice)), amount, opt.account, contract, args...)
	if err != nil {
		return xerrors.Errorf("deploying new BEvm contract: %v", err)
	}

	err = writeAccountFile(opt.account, opt.accountName, false)
	if err != nil {
		return xerrors.Errorf("writing account file: %v", err)
	}

	err = writeContractFile(contractInstance, abiFilepath, contractName, true)
	if err != nil {
		return xerrors.Errorf("writing contract file: %v", err)
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "%s deployed\n", contractInstance)
	if err != nil {
		return xerrors.Errorf("writing report msg: %v", err)
	}

	return nil
}

func executeTransaction(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return xerrors.Errorf("handling options: %v", err)
	}

	gasLimit := ctx.Uint64("gas-limit")
	gasPrice := ctx.Uint64("gas-price")
	amount := ctx.Uint64("amount")
	contractName := ctx.String("contract-name")

	contractInstance, err := readContractFile(contractName)
	if err != nil {
		return xerrors.Errorf("reading contract file: %v", err)
	}

	if ctx.NArg() == 0 {
		return xerrors.New("missing transaction name argument")
	}

	method := ctx.Args().First()
	methodAbi, ok := contractInstance.Parent.Abi.Methods[method]
	if !ok {
		return xerrors.Errorf("transaction \"%s\" does not exist for this contract", method)
	}

	userArgs := ctx.Args().Tail()
	args, err := decodeEvmArgs(userArgs, methodAbi.Inputs)
	if err != nil {
		return xerrors.Errorf("decoding contract transaction arguments: %v", err)
	}

	// Perform command

	err = opt.bevmClient.Transaction(
		gasLimit, big.NewInt(int64(gasPrice)), amount, opt.account,
		contractInstance, method, args...)
	if err != nil {
		return xerrors.Errorf("executing contract transaction: %v", err)
	}

	err = writeAccountFile(opt.account, opt.accountName, false)
	if err != nil {
		return xerrors.Errorf("writing account file: %v", err)
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "transaction executed\n")
	if err != nil {
		return xerrors.Errorf("writing report msg: %v", err)
	}

	return nil
}

func executeCall(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return xerrors.Errorf("handling options: %v", err)
	}

	contractName := ctx.String("contract-name")

	contractInstance, err := readContractFile(contractName)
	if err != nil {
		return xerrors.Errorf("reading contract file: %v", err)
	}

	if ctx.NArg() == 0 {
		return xerrors.New("missing view method name argument")
	}

	method := ctx.Args().First()
	methodAbi, ok := contractInstance.Parent.Abi.Methods[method]
	if !ok {
		return xerrors.Errorf("view method \"%s\" does not exist for this contract", method)
	}

	userArgs := ctx.Args().Tail()
	args, err := decodeEvmArgs(userArgs, methodAbi.Inputs)
	if err != nil {
		return xerrors.Errorf("decoding contract view method arguments: %v", err)
	}

	// Perform command

	result, err := opt.bevmClient.Call(opt.account, contractInstance, method, args...)
	if err != nil {
		return xerrors.Errorf("executing view method: %v", err)
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "call return value: %v [%s]\n", result, reflect.TypeOf(result))
	if err != nil {
		return xerrors.Errorf("writing report msg: %v", err)
	}

	return nil
}
