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
	"strings"
	"time"

	"golang.org/x/xerrors"

	cli "github.com/urfave/cli"
	"go.dedis.ch/cothority/v4/bevm"
	"go.dedis.ch/cothority/v4/byzcoin"
	"go.dedis.ch/cothority/v4/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v4/darc"
	"go.dedis.ch/onet/v4/cfgpath"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"

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
				Usage: "account name; default is the account address in hex",
			},
		},
		Action: createAccount,
	},
	{
		Name:      "credit_account",
		Usage:     "credit a BEvm account",
		Aliases:   []string{"ma"},
		ArgsUsage: "",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "bc",
				EnvVar:   "BC",
				Required: true,
				Usage:    "ByzCoin config to use",
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
				Name:  "amount",
				Value: 5,
				Usage: "amount in Ether with which to credit the account",
			},
		},
		Action: creditAccount,
	},
	{
		Name:      "deploy_contract",
		Usage:     "deploy a BEvm contract",
		Aliases:   []string{"dc"},
		ArgsUsage: "",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "bc",
				EnvVar:   "BC",
				Required: true,
				Usage:    "ByzCoin config to use",
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
				Name:     "abi",
				Usage:    "contract ABI file",
				Required: true,
			},
			cli.StringFlag{
				Name:     "bin",
				Usage:    "contract bytecode file",
				Required: true,
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
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use (required)",
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
			cli.StringFlag{
				Name:     "method",
				Usage:    "contract method name",
				Required: true,
			},
		},
		Action: executeTransaction,
	},
	{
		Name:      "call",
		Usage:     "call a view method on a BEvm contract instance",
		Aliases:   []string{"xc"},
		ArgsUsage: "",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use (required)",
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
			cli.StringFlag{
				Name:     "method",
				Usage:    "contract method name",
				Required: true,
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

func spawn(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	signer, err := lib.LoadKey(cfg.AdminIdentity)
	if err != nil {
		return err
	}

	bevmInstID, err := bevm.NewBEvm(cl, *signer, &cfg.AdminDarc)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.App.Writer, "Created BEvm instance with ID: %s\n", bevmInstID)
	if err != nil {
		return err
	}

	return nil
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

	if name == "" {
		name = account.Address.String()
	}

	fmt.Printf("New account \"%s\" created at address: %s\n", name, account.Address.String())

	return writeAccountFile(account, name)
}

func creditAccount(ctx *cli.Context) error {
	// Retrieve arguments

	bcArg := ctx.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	bevmID, err := hex.DecodeString(ctx.String("bevm-id"))
	if err != nil {
		return err
	}

	account, err := readAccountFile(ctx.String("account-name"))
	if err != nil {
		return err
	}

	amount := ctx.Uint64("amount")

	// Perform command

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	signer, err := lib.LoadKey(cfg.AdminIdentity)
	if err != nil {
		return err
	}

	bevmClient, err := bevm.NewClient(cl, *signer, byzcoin.NewInstanceID(bevmID))
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

func deployContract(ctx *cli.Context) error {
	// Retrieve arguments
	bcArg := ctx.String("bc")
	bevmID := ctx.String("bevm-id")
	accountName := ctx.String("account-name")
	gasLimit := ctx.Uint64("gas-limit")
	gasPrice := ctx.Uint64("gas-price")
	amount := ctx.Uint64("amount")
	abiFilepath := ctx.String("abi")
	binFilepath := ctx.String("bin")

	// Perform command

	account, err := readAccountFile(accountName)
	if err != nil {
		return err
	}

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

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	signer, err := lib.LoadKey(cfg.AdminIdentity)
	if err != nil {
		return err
	}

	bevmIID, err := hex.DecodeString(bevmID)
	if err != nil {
		return err
	}

	bevmClient, err := bevm.NewClient(cl, *signer, byzcoin.NewInstanceID(bevmIID))
	if err != nil {
		return err
	}

	args, err := decodeArgs(ctx.Args(), contract.Abi.Constructor.Inputs)
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
	// Retrieve arguments
	bcArg := ctx.String("bc")
	bevmID := ctx.String("bevm-id")
	accountName := ctx.String("account-name")
	gasLimit := ctx.Uint64("gas-limit")
	gasPrice := ctx.Uint64("gas-price")
	amount := ctx.Uint64("amount")
	contractName := ctx.String("contract-name")
	method := ctx.String("method")

	// Perform command

	account, err := readAccountFile(accountName)
	if err != nil {
		return err
	}

	contractInstance, err := readContractFile(contractName)
	if err != nil {
		return err
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	signer, err := lib.LoadKey(cfg.AdminIdentity)
	if err != nil {
		return err
	}

	bevmIID, err := hex.DecodeString(bevmID)
	if err != nil {
		return err
	}

	bevmClient, err := bevm.NewClient(cl, *signer, byzcoin.NewInstanceID(bevmIID))
	if err != nil {
		return err
	}

	methodAbi, ok := contractInstance.Parent.Abi.Methods[method]
	if !ok {
		return xerrors.Errorf("Method \"%s\" does not exist for this contract", method)
	}
	args, err := decodeArgs(ctx.Args(), methodAbi.Inputs)
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
	// Retrieve arguments
	bcArg := ctx.String("bc")
	bevmID := ctx.String("bevm-id")
	accountName := ctx.String("account-name")
	contractName := ctx.String("contract-name")
	method := ctx.String("method")

	// Perform command

	account, err := readAccountFile(accountName)
	if err != nil {
		return err
	}

	contractInstance, err := readContractFile(contractName)
	if err != nil {
		return err
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	signer, err := lib.LoadKey(cfg.AdminIdentity)
	if err != nil {
		return err
	}

	bevmIID, err := hex.DecodeString(bevmID)
	if err != nil {
		return err
	}

	bevmClient, err := bevm.NewClient(cl, *signer, byzcoin.NewInstanceID(bevmIID))
	if err != nil {
		return err
	}

	methodAbi, ok := contractInstance.Parent.Abi.Methods[method]
	if !ok {
		return xerrors.Errorf("Method \"%s\" does not exist for this contract", method)
	}
	args, err := decodeArgs(ctx.Args(), methodAbi.Inputs)
	if err != nil {
		return err
	}

	// FIXME: handle multiple types
	// result := getResultVariable()
	result, err := bevmClient.Call(account, contractInstance, method, args...)
	// result, err := decodeResult(func(result interface{}) error {
	// 	return bevmClient.Call(account, &result, contractInstance, method, args...)
	// }, methodAbi.Outputs)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(ctx.App.Writer, "call executed; return value: %v\n", result)
	if err != nil {
		return err
	}

	return nil
}
