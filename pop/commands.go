package main

import "gopkg.in/urfave/cli.v1"

/*
This holds the cli-commands so the main-file is less cluttered.
*/

var commandOrg, commandClient cli.Command

func init() {
	commandOrg = cli.Command{
		Name:  "org",
		Usage: "Organising a PoParty",
		Subcommands: []cli.Command{
			{
				Name:      "link",
				Aliases:   []string{"l"},
				Usage:     "link to a cothority",
				ArgsUsage: "IP-address:port",
				Action:    orgLink,
			},
			{
				Name:      "config",
				Aliases:   []string{"c"},
				Usage:     "stores the configuration",
				ArgsUsage: "pop_desc.toml group.toml",
				Action:    orgConfig,
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
		},
	}
}
