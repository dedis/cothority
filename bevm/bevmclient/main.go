package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"golang.org/x/xerrors"

	cli "github.com/urfave/cli"
	"go.dedis.ch/cothority/v3/bevm"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func init() {
	network.RegisterMessages(&darc.Darc{}, &darc.Identity{}, &darc.Signer{})
}

var cmds = cli.Commands{
	{
		Name:      "create_account",
		Usage:     "create a new BEvm account",
		Aliases:   []string{"ca"},
		ArgsUsage: "",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "account-name",
				Value: "account",
				Usage: "account name",
			},
		},
		Action: createAccount,
	},
	{
		Name:      "credit_account",
		Usage:     "credit a BEvm account",
		Aliases:   []string{"ma"},
		ArgsUsage: "<amount in Ether>",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "bc",
				EnvVar:   "BC",
				Usage:    "ByzCoin config to use",
				Required: true,
			},
			cli.StringFlag{
				Name:     "bevm-id",
				Usage:    "BEvm instance ID to use",
				Required: true,
			},
			cli.StringFlag{
				Name:  "account-name",
				Value: "account",
				Usage: "account name",
			},
		},
		Action: creditAccount,
	},
	{
		Name:      "get_account_balance",
		Usage:     "retrieve the balance of a BEvm account",
		Aliases:   []string{"ba"},
		ArgsUsage: "",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "bc",
				EnvVar:   "BC",
				Usage:    "ByzCoin config to use",
				Required: true,
			},
			cli.StringFlag{
				Name:     "bevm-id",
				Usage:    "BEvm instance ID to use",
				Required: true,
			},
			cli.StringFlag{
				Name:  "account-name",
				Value: "account",
				Usage: "account name",
			},
		},
		Action: getAccountBalance,
	},
	{
		Name:      "deploy_contract",
		Usage:     "deploy a BEvm contract",
		Aliases:   []string{"dc"},
		ArgsUsage: "<abi file> <bytecode file>",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "bc",
				EnvVar:   "BC",
				Usage:    "ByzCoin config to use",
				Required: true,
			},
			cli.StringFlag{
				Name:     "bevm-id",
				Usage:    "BEvm instance ID to use",
				Required: true,
			},
			cli.StringFlag{
				Name:  "account-name",
				Value: "account",
				Usage: "account name",
			},
			cli.Uint64Flag{
				Name:  "gas-limit",
				Value: 1e7,
				Usage: "gas limit for the transaction",
			},
			cli.Uint64Flag{
				Name:  "gas-price",
				Value: 1,
				Usage: "gas price for the transaction",
			},
			cli.Uint64Flag{
				Name:  "amount",
				Value: 0,
				Usage: "amount in Ether to send to the contract once deployed",
			},
		},
		Action: deployContract,
	},
	{
		Name:      "transaction",
		Usage:     "execute a transaction on a BEvm contract instance",
		Aliases:   []string{"xt"},
		ArgsUsage: "",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "bc",
				EnvVar:   "BC",
				Usage:    "ByzCoin config to use",
				Required: true,
			},
			cli.StringFlag{
				Name:     "bevm-id",
				Usage:    "BEvm instance ID to use",
				Required: true,
			},
			cli.StringFlag{
				Name:  "account-name",
				Value: "account",
				Usage: "account name",
			},
			cli.Uint64Flag{
				Name:  "gas-limit",
				Value: 1e7,
				Usage: "gas limit for the transaction",
			},
			cli.Uint64Flag{
				Name:  "gas-price",
				Value: 1,
				Usage: "gas price for the transaction",
			},
			cli.Uint64Flag{
				Name:  "amount",
				Value: 0,
				Usage: "amount in Ether to send to the contract once deployed",
			},
			cli.StringFlag{
				Name:  "contract-name",
				Value: "contract",
				Usage: "contract name",
			},
		},
		Action: executeTransaction,
	},
	{
		Name:      "call",
		Usage:     "call a view method on a BEvm contract instance",
		Aliases:   []string{"xc"},
		ArgsUsage: "<methodname> [<arg>...]",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "bc",
				EnvVar:   "BC",
				Usage:    "ByzCoin config to use",
				Required: true,
			},
			cli.StringFlag{
				Name:     "bevm-id",
				Usage:    "BEvm instance ID to use",
				Required: true,
			},
			cli.StringFlag{
				Name:  "account-name",
				Value: "account",
				Usage: "account name",
			},
			cli.StringFlag{
				Name:  "contract-name",
				Value: "contract",
				Usage: "contract name",
			},
		},
		Action: executeCall,
	},
}

var cliApp = cli.NewApp()

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

var gitTag = "dev"

func init() {
	cliApp.Name = "bevmclient"
	cliApp.Usage = "Manage BEvm accounts and contracts."
	cliApp.Version = gitTag
	cliApp.Commands = cmds
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:   "config, c",
			EnvVar: "BC_CONFIG",
			Value:  getDataPath(cliApp.Name),
			Usage:  "path to configuration-directory",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		lib.ConfigPath = c.String("config")
		return nil
	}
}

func main() {
	rand.Seed(time.Now().Unix())
	err := cliApp.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	return
}

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

func createAccount(ctx *cli.Context) error {
	// Retrieve arguments

	name := ctx.String("name")

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

	return writeAccountFile(account, name)
}

func creditAccount(ctx *cli.Context) error {
	// Retrieve options and arguments

	bcFile := ctx.String("bc")
	bevmIDStr := ctx.String("bevm-id")
	accountName := ctx.String("account-name")

	bevmID, err := hex.DecodeString(bevmIDStr)
	if err != nil {
		return err
	}

	account, err := readAccountFile(accountName)
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

	bevmClient, err := getBevmClient(bcFile, bevmID)
	if err != nil {
		return err
	}

	err = bevmClient.CreditAccount(big.NewInt(int64(amount*bevm.WeiPerEther)), account.Address)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "Credited account %s with %d Ether\n",
		account.Address.Hex(), amount)
	if err != nil {
		return err
	}

	return nil
}

func getAccountBalance(ctx *cli.Context) error {
	// Retrieve options and arguments

	bcFile := ctx.String("bc")
	bevmIDStr := ctx.String("bevm-id")
	accountName := ctx.String("account-name")

	bevmID, err := hex.DecodeString(bevmIDStr)
	if err != nil {
		return err
	}

	account, err := readAccountFile(accountName)
	if err != nil {
		return err
	}

	// Perform command

	bevmClient, err := getBevmClient(bcFile, bevmID)
	if err != nil {
		return err
	}

	amount, err := bevmClient.GetAccountBalance(account.Address)
	if err != nil {
		return err
	}

	var amountEther, amountWei big.Int
	amountEther.DivMod(amount, big.NewInt(bevm.WeiPerEther), &amountWei)
	_, err = fmt.Fprintf(ctx.App.Writer, "Balance of account %s: %v Ether, %v Wei\n",
		account.Address.Hex(), amountEther.String(), amountWei.String())
	if err != nil {
		return err
	}

	return nil
}

func deployContract(ctx *cli.Context) error {
	// Retrieve options and arguments

	bcFile := ctx.String("bc")
	bevmIDStr := ctx.String("bevm-id")
	accountName := ctx.String("account-name")
	gasLimit := ctx.Uint64("gas-limit")
	gasPrice := ctx.Uint64("gas-price")
	amount := ctx.Uint64("amount")

	bevmID, err := hex.DecodeString(bevmIDStr)
	if err != nil {
		return err
	}

	account, err := readAccountFile(accountName)
	if err != nil {
		return err
	}

	if ctx.NArg() != 2 {
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
	args, err := decodeArgs(userArgs, contract.Abi.Constructor.Inputs)
	if err != nil {
		return err
	}

	// Perform command

	bevmClient, err := getBevmClient(bcFile, bevmID)
	if err != nil {
		return err
	}

	contractInstance, err := bevmClient.Deploy(gasLimit, big.NewInt(int64(gasPrice)), amount, account, contract, args...)
	if err != nil {
		return err
	}

	err = writeAccountFile(account, accountName)
	if err != nil {
		return err
	}

	err = writeContractFile(contractInstance, abiFilepath, "contract")
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

	bcFile := ctx.String("bc")
	bevmIDStr := ctx.String("bevm-id")
	accountName := ctx.String("account-name")
	gasLimit := ctx.Uint64("gas-limit")
	gasPrice := ctx.Uint64("gas-price")
	amount := ctx.Uint64("amount")
	contractName := ctx.String("contract-name")

	bevmID, err := hex.DecodeString(bevmIDStr)
	if err != nil {
		return err
	}

	account, err := readAccountFile(accountName)
	if err != nil {
		return err
	}

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
	args, err := decodeArgs(userArgs, methodAbi.Inputs)
	if err != nil {
		return err
	}

	// Perform command

	bevmClient, err := getBevmClient(bcFile, bevmID)
	if err != nil {
		return err
	}

	err = bevmClient.Transaction(gasLimit, big.NewInt(int64(gasPrice)), amount, account, contractInstance, method, args...)
	if err != nil {
		return err
	}

	err = writeAccountFile(account, accountName)
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

	bcFile := ctx.String("bc")
	bevmIDStr := ctx.String("bevm-id")
	accountName := ctx.String("account-name")
	contractName := ctx.String("contract-name")

	bevmID, err := hex.DecodeString(bevmIDStr)
	if err != nil {
		return err
	}

	account, err := readAccountFile(accountName)
	if err != nil {
		return err
	}

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
	args, err := decodeArgs(userArgs, methodAbi.Inputs)
	if err != nil {
		return err
	}

	// Perform command

	bevmClient, err := getBevmClient(bcFile, bevmID)
	if err != nil {
		return err
	}

	result, err := bevmClient.Call(account, contractInstance, method, args...)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "call return value: %v [%s]\n", result, reflect.TypeOf(result))
	if err != nil {
		return err
	}

	return nil
}
