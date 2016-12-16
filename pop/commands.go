package main

import "gopkg.in/urfave/cli.v1"

/*
This holds the cli-commands so the main-file is less cluttered.
*/

var commandMgr, commandClient cli.Command

func init() {
	commandMgr = cli.Command{
		Name:  "mgr",
		Usage: "Managing a PoParty",
		Subcommands: []cli.Command{
			{
				Name:      "link",
				Aliases:   []string{"l"},
				Usage:     "link to a cothority",
				ArgsUsage: "IP-address:port",
				Action:    mgrLink,
			},
			{
				Name:      "config",
				Aliases:   []string{"c"},
				Usage:     "stores the configuration",
				ArgsUsage: "pop_desc.toml group.toml",
				Action:    mgrConfig,
			},
			{
				Name:    "public",
				Aliases: []string{"p"},
				Usage:   "stores a public key during the party",
				Action:  mgrPublic,
			},
			{
				Name:    "final",
				Aliases: []string{"f"},
				Usage:   "finalizes the party",
				Action:  mgrFinal,
			},
		},
	}
	commandClient = cli.Command{
		Name:  "client",
		Usage: "client for a pop-party",
		Subcommands: []cli.Command{
			{
				Name:    "create",
				Aliases: []string{"cr"},
				Usage:   "create a private/public key pair",
				Action:  clientCreate,
			},
			{
				Name:    "join",
				Aliases: []string{"j"},
				Usage:   "joins a poparty",
				Action:  clientJoin,
			},
			{
				Name:      "sign",
				Aliases:   []string{"s"},
				Usage:     "sign a message and its context",
				ArgsUsage: "message context",
				Action:    clientSign,
			},
			{
				Name:      "verify",
				Aliases:   []string{"v"},
				Usage:     "verifies a tag and a signature",
				ArgsUsage: "message context tag signature",
				Action:    clientVerify,
			},
		},
	}
}
