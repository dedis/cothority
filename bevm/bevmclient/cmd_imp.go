package main

import (
	"errors"
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
		return err // FIXME: use xerrors
	}

	account, err := bevm.NewEvmAccount(common.Bytes2Hex(crypto.FromECDSA(pk)))
	if err != nil {
		return err
	}

	fmt.Printf("New account \"%s\" created at address: %s\n", name, account.Address.String())

	return writeAccountFile(account, name, true)
}

func creditAccount(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return err
	}

	if !ctx.Args().Present() {
		return errors.New("Missing amount value")
	}

	amountStr := ctx.Args().First()
	amount, err := strconv.ParseUint(amountStr, 0, 64)
	if err != nil {
		return err
	}

	// Perform command

	amountBig := big.NewInt(int64(amount))
	amountBig.Mul(amountBig, big.NewInt(bevm.WeiPerEther))

	err = opt.bevmClient.CreditAccount(amountBig, opt.account.Address)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "Credited account %s with %d Ether\n",
		opt.account.Address.Hex(), amount)
	if err != nil {
		return err
	}

	return nil
}

func getAccountBalance(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return err
	}

	// Perform command

	amount, err := opt.bevmClient.GetAccountBalance(opt.account.Address)
	if err != nil {
		return err
	}

	var amountEther, amountWei big.Int
	amountEther.DivMod(amount, big.NewInt(bevm.WeiPerEther), &amountWei)
	_, err = fmt.Fprintf(ctx.App.Writer, "Balance of account %s: %v Ether, %v Wei\n",
		opt.account.Address.Hex(), amountEther.String(), amountWei.String())
	if err != nil {
		return err
	}

	return nil
}

func deployContract(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return err
	}

	gasLimit := ctx.Uint64("gas-limit")
	gasPrice := ctx.Uint64("gas-price")
	amount := ctx.Uint64("amount")

	if ctx.NArg() < 2 {
		return errors.New("Missing some argument")
	}

	abiFilepath := ctx.Args().Get(0)
	binFilepath := ctx.Args().Get(1)

	abiData, err := ioutil.ReadFile(abiFilepath)
	if err != nil {
		return errors.New("error reading contract ABI: " + err.Error())
	}

	binData, err := ioutil.ReadFile(binFilepath)
	if err != nil {
		return errors.New("error reading contract Bytecode: " + err.Error())
	}

	contract, err := bevm.NewEvmContract("newContract", string(abiData), string(binData))
	if err != nil {
		return err
	}

	userArgs := ctx.Args()[2:]
	args, err := decodeEvmArgs(userArgs, contract.Abi.Constructor.Inputs)
	if err != nil {
		return err
	}

	// Perform command

	contractInstance, err := opt.bevmClient.Deploy(
		gasLimit, big.NewInt(int64(gasPrice)), amount, opt.account, contract, args...)
	if err != nil {
		return err
	}

	err = writeAccountFile(opt.account, opt.accountName, false)
	if err != nil {
		return err
	}

	err = writeContractFile(contractInstance, abiFilepath, "contract", true)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "%s deployed\n", contractInstance)
	if err != nil {
		return err
	}

	return nil
}

func executeTransaction(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return err
	}

	gasLimit := ctx.Uint64("gas-limit")
	gasPrice := ctx.Uint64("gas-price")
	amount := ctx.Uint64("amount")
	contractName := ctx.String("contract-name")

	contractInstance, err := readContractFile(contractName)
	if err != nil {
		return err
	}

	if ctx.NArg() == 0 {
		return errors.New("Missing method name")
	}

	method := ctx.Args().First()
	methodAbi, ok := contractInstance.Parent.Abi.Methods[method]
	if !ok {
		return xerrors.Errorf("Method \"%s\" does not exist for this contract", method)
	}

	userArgs := ctx.Args().Tail()
	args, err := decodeEvmArgs(userArgs, methodAbi.Inputs)
	if err != nil {
		return err
	}

	// Perform command

	err = opt.bevmClient.Transaction(
		gasLimit, big.NewInt(int64(gasPrice)), amount, opt.account, contractInstance, method, args...)
	if err != nil {
		return err
	}

	err = writeAccountFile(opt.account, opt.accountName, false)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "transaction executed\n")
	if err != nil {
		return err
	}

	return nil
}

func executeCall(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return err
	}

	contractName := ctx.String("contract-name")

	contractInstance, err := readContractFile(contractName)
	if err != nil {
		return err
	}

	if ctx.NArg() == 0 {
		return errors.New("Missing method name")
	}

	method := ctx.Args().First()
	methodAbi, ok := contractInstance.Parent.Abi.Methods[method]
	if !ok {
		return xerrors.Errorf("Method \"%s\" does not exist for this contract", method)
	}

	userArgs := ctx.Args().Tail()
	args, err := decodeEvmArgs(userArgs, methodAbi.Inputs)
	if err != nil {
		return err
	}

	// Perform command

	result, err := opt.bevmClient.Call(opt.account, contractInstance, method, args...)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "call return value: %v [%s]\n", result, reflect.TypeOf(result))
	if err != nil {
		return err
	}

	return nil
}
