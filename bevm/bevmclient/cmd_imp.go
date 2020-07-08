package main

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"strconv"

	"golang.org/x/xerrors"

	cli "github.com/urfave/cli"
	"go.dedis.ch/cothority/v3/bevm"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func createAccount(ctx *cli.Context) error {
	// Retrieve arguments

	name := ctx.String("accountName")

	// Perform command

	pk, err := crypto.GenerateKey()
	if err != nil {
		return xerrors.Errorf("failed to generate EVM account key: %v", err)
	}

	account, err := bevm.NewEvmAccount(common.Bytes2Hex(crypto.FromECDSA(pk)))
	if err != nil {
		return xerrors.Errorf("failed to create new EVM account: %v", err)
	}

	fmt.Printf("New account \"%s\" created at address: %s\n",
		name, account.Address.String())

	return writeAccountFile(account, name, true)
}

func creditAccount(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return xerrors.Errorf("failed to handle provided options: %v", err)
	}

	if !ctx.Args().Present() {
		return xerrors.New("missing <amount> argument")
	}

	amountStr := ctx.Args().First()
	amount, err := strconv.ParseUint(amountStr, 0, 64)
	if err != nil {
		return xerrors.Errorf("failed to parse <amount> value: %v", err)
	}

	// Perform command

	amountBig := big.NewInt(int64(amount))
	amountBig.Mul(amountBig, big.NewInt(bevm.WeiPerEther))

	_, err = opt.bevmClient.CreditAccount(amountBig, opt.account.Address)
	if err != nil {
		return xerrors.Errorf("failed to credit BEvm account: %v", err)
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "Credited account %s with %d Ether\n",
		opt.account.Address.Hex(), amount)
	if err != nil {
		return xerrors.Errorf("failed to write report msg: %v", err)
	}

	return nil
}

func getAccountBalance(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return xerrors.Errorf("failed to handle provided options: %v", err)
	}

	// Perform command

	amount, err := opt.bevmClient.GetAccountBalance(opt.account.Address)
	if err != nil {
		return xerrors.Errorf("failed to retrieve BEvm account "+
			"balance: %v", err)
	}

	var amountEther, amountWei big.Int
	amountEther.DivMod(amount, big.NewInt(bevm.WeiPerEther), &amountWei)
	_, err = fmt.Fprintf(ctx.App.Writer,
		"Balance of account %s: %v Ether, %v Wei\n",
		opt.account.Address.Hex(), amountEther.String(), amountWei.String())
	if err != nil {
		return xerrors.Errorf("failed to write report msg: %v", err)
	}

	return nil
}

func deployContract(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return xerrors.Errorf("failed to handle provided options: %v", err)
	}

	gasLimit := ctx.Uint64("gasLimit")
	gasPrice := ctx.Uint64("gasPrice")
	amount := ctx.Uint64("amount")
	contractName := ctx.String("contractName")

	if ctx.NArg() < 2 {
		return xerrors.Errorf("missing some argument (expected at least 2, "+
			"got %d)", ctx.NArg())
	}

	abiFilepath := ctx.Args().Get(0)
	binFilepath := ctx.Args().Get(1)

	abiData, err := ioutil.ReadFile(abiFilepath)
	if err != nil {
		return xerrors.Errorf("failed to read contract ABI from "+
			"provided file: %v", err)
	}

	binData, err := ioutil.ReadFile(binFilepath)
	if err != nil {
		return xerrors.Errorf("failed to read contract Bytecode "+
			"from provided file: %v", err)
	}

	contract, err := bevm.NewEvmContract("newContract",
		string(abiData), string(binData))
	if err != nil {
		return xerrors.Errorf("failed to create new BEvm contract: %v", err)
	}

	constrAbi := contract.Abi.Constructor
	userArgs := ctx.Args()[2:]
	if len(userArgs) != len(constrAbi.Inputs) {
		return xerrors.Errorf("wrong number of arguments for contract "+
			"constructor: expected %d, got %d",
			len(constrAbi.Inputs), len(userArgs))
	}
	args, err := bevm.DecodeEvmArgs(userArgs, constrAbi.Inputs)
	if err != nil {
		return xerrors.Errorf("failed to decode contract constructor "+
			"arguments: %v", err)
	}

	// Perform command

	_, contractInstance, err := opt.bevmClient.Deploy(
		gasLimit, gasPrice, amount, opt.account, contract, args...)
	if err != nil {
		return xerrors.Errorf("failed to deploy new BEvm contract: %v", err)
	}

	err = writeAccountFile(opt.account, opt.accountName, false)
	if err != nil {
		return xerrors.Errorf("failed to save account information: %v", err)
	}

	err = writeContractFile(contractInstance, abiFilepath, contractName, true)
	if err != nil {
		return xerrors.Errorf("failed to save contract information: %v", err)
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "%s deployed\n", contractInstance)
	if err != nil {
		return xerrors.Errorf("failed to write report msg: %v", err)
	}

	return nil
}

func executeTransaction(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return xerrors.Errorf("failed to handle pprovided options: %v", err)
	}

	gasLimit := ctx.Uint64("gasLimit")
	gasPrice := ctx.Uint64("gasPrice")
	amount := ctx.Uint64("amount")
	contractName := ctx.String("contractName")

	contractInstance, err := readContractFile(contractName)
	if err != nil {
		return xerrors.Errorf("failed to load contract information: %v", err)
	}

	if ctx.NArg() == 0 {
		return xerrors.New("missing transaction name argument")
	}

	method := ctx.Args().First()
	methodAbi, ok := contractInstance.Parent.Abi.Methods[method]
	if !ok {
		return xerrors.Errorf("transaction \"%s\" does not exist "+
			"for this contract", method)
	}

	userArgs := ctx.Args().Tail()
	if len(userArgs) != len(methodAbi.Inputs) {
		return xerrors.Errorf("wrong number of arguments for \"%s\": "+
			"expected %d, got %d", method, len(methodAbi.Inputs), len(userArgs))
	}
	args, err := bevm.DecodeEvmArgs(userArgs, methodAbi.Inputs)
	if err != nil {
		return xerrors.Errorf("failed to decode contract transaction "+
			"arguments: %v", err)
	}

	// Perform command

	_, err = opt.bevmClient.Transaction(
		gasLimit, gasPrice, amount, opt.account, contractInstance,
		method, args...)
	if err != nil {
		return xerrors.Errorf("failed to execute contract transaction: %v", err)
	}

	err = writeAccountFile(opt.account, opt.accountName, false)
	if err != nil {
		return xerrors.Errorf("failed to save account information: %v", err)
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "transaction executed\n")
	if err != nil {
		return xerrors.Errorf("failed to write report msg: %v", err)
	}

	return nil
}

func executeCall(ctx *cli.Context) error {
	// Retrieve options and arguments

	opt, err := handleCommonOptions(ctx)
	if err != nil {
		return xerrors.Errorf("failed to handle provided options: %v", err)
	}

	contractName := ctx.String("contractName")

	contractInstance, err := readContractFile(contractName)
	if err != nil {
		return xerrors.Errorf("failed to load contract information: %v", err)
	}

	if ctx.NArg() == 0 {
		return xerrors.New("missing view method name argument")
	}

	method := ctx.Args().First()
	methodAbi, ok := contractInstance.Parent.Abi.Methods[method]
	if !ok {
		return xerrors.Errorf("view method \"%s\" does not exist for this "+
			"contract", method)
	}
	if !methodAbi.Const {
		return xerrors.Errorf("callable \"%s\" is not a view method", method)
	}

	userArgs := ctx.Args().Tail()
	if len(userArgs) != len(methodAbi.Inputs) {
		return xerrors.Errorf("wrong number of arguments for \"%s\": "+
			"expected %d, got %d", method, len(methodAbi.Inputs), len(userArgs))
	}
	args, err := bevm.DecodeEvmArgs(userArgs, methodAbi.Inputs)
	if err != nil {
		return xerrors.Errorf("failed to decode contract view method "+
			"arguments: %v", err)
	}

	// Perform command

	result, err := opt.bevmClient.Call(opt.account, contractInstance,
		method, args...)
	if err != nil {
		return xerrors.Errorf("failed to execute view method: %v", err)
	}

	resultJSON, err := bevm.EncodeEvmResult(result, methodAbi.Outputs)
	if err != nil {
		return xerrors.Errorf("failed to encode view method result: %v", err)
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "call result: %v\n", resultJSON)
	if err != nil {
		return xerrors.Errorf("failed to write report msg: %v", err)
	}

	return nil
}
