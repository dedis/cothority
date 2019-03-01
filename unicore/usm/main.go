package main

import (
	"os"

	"go.dedis.ch/onet/v3/log"
	cli "gopkg.in/urfave/cli.v1"
)

var cmds = cli.Commands{
	{
		Name:    "create",
		Usage:   "create a smart contract",
		Aliases: []string{"c"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "key",
				EnvVar: "PRIVATE_KEY",
				Usage:  "the ed25519 private key that will sign the create transaction",
			},
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config",
			},
			cli.StringFlag{
				Name:   "instance",
				EnvVar: "INSTANCE_ID",
				Usage:  "The instance ID",
			},
		},
		Action: create,
	},
	{
		Name:    "exec",
		Usage:   "execute a smart contract",
		Aliases: []string{"e"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "key",
				EnvVar: "PRIVATE_KEY",
				Usage:  "the ed25519 private key that will sign the create transaction",
			},
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config",
			},
			cli.StringFlag{
				Name:   "instance",
				EnvVar: "INSTANCE_ID",
				Usage:  "The instance ID",
			},
		},
		Action: exec,
	},
}

var cliApp = cli.NewApp()

func init() {
	cliApp.Name = "usm"
	cliApp.Usage = "Create smart contracts and use them"
	cliApp.Version = "0.1"
	cliApp.Commands = cmds
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
}

func main() {
	log.ErrFatal(cliApp.Run(os.Args))
}
