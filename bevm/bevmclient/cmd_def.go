package main

import (
	cli "github.com/urfave/cli"
)

var commonFlags = []cli.Flag{
	cli.StringFlag{
		Name:     "bc",
		EnvVar:   "BC",
		Usage:    "ByzCoin config to use",
		Required: true,
	},
	cli.StringFlag{
		Name:     "bevm-id",
		EnvVar:   "BEVM_ID",
		Usage:    "BEvm instance ID to use",
		Required: true,
	},
	cli.StringFlag{
		Name:  "sign",
		Usage: "public key of the signing entity (default is the admin public key)",
	},
	cli.StringFlag{
		Name:  "account-name",
		Value: "account",
		Usage: "account name",
	},
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
		Flags:     commonFlags,
		Action:    creditAccount,
	},
	{
		Name:      "get_account_balance",
		Usage:     "retrieve the balance of a BEvm account",
		Aliases:   []string{"ba"},
		ArgsUsage: "",
		Flags:     commonFlags,
		Action:    getAccountBalance,
	},
	{
		Name:      "deploy_contract",
		Usage:     "deploy a BEvm contract",
		Aliases:   []string{"dc"},
		ArgsUsage: "<abi file> <bytecode file> [<arg>...]",
		Flags: append(commonFlags,
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
		),
		Action: deployContract,
	},
	{
		Name:      "transaction",
		Usage:     "execute a transaction on a BEvm contract instance",
		Aliases:   []string{"xt"},
		ArgsUsage: "<transaction name> [<arg>...]",
		Flags: append(commonFlags,
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
		),
		Action: executeTransaction,
	},
	{
		Name:      "call",
		Usage:     "call a view method on a BEvm contract instance",
		Aliases:   []string{"xc"},
		ArgsUsage: "<view method name> [<arg>...]",
		Flags: append(commonFlags,
			cli.StringFlag{
				Name:  "contract-name",
				Value: "contract",
				Usage: "contract name",
			},
		),
		Action: executeCall,
	},
}
