package main

import "gopkg.in/urfave/cli.v1"

/*
This holds the cli-commands so the main-file is less cluttered.
*/

var commandOrg, commandAttendee, commandAuth cli.Command

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
				ArgsUsage: "pop_desc.toml [merged_party.toml]",
				Action:    orgConfig,
			},
			{
				Name:      "public",
				Aliases:   []string{"p"},
				Usage:     "stores a public key during the party",
				ArgsUsage: "party_hash",
				Action:    orgPublic,
			},
			{
				Name:      "final",
				Aliases:   []string{"f"},
				Usage:     "finalizes the party",
				ArgsUsage: "party_hash",
				Action:    orgFinal,
			},
			{
				Name:      "merge",
				Aliases:   []string{"m"},
				Usage:     "starts merging process",
				ArgsUsage: "party_hash",
				Action:    orgMerge,
			},
		},
	}

	commandAttendee = cli.Command{
		Name:  "attendee",
		Usage: "attendee of a pop-party",
		Subcommands: []cli.Command{
			{
				Name:    "create",
				Aliases: []string{"cr"},
				Usage:   "create a private/public key pair",
				Action:  attCreate,
			},
			{
				Name:      "join",
				Aliases:   []string{"j"},
				Usage:     "join a poparty",
				ArgsUsage: "party_hash",
				Action:    attJoin,
				Flags: []cli.Flag{
					cli.BoolTFlag{
						Name:  "yes,y",
						Usage: "disable asking",
					},
				},
			},
			{
				Name:      "sign",
				Aliases:   []string{"s"},
				Usage:     "sign a message and its context",
				ArgsUsage: "message context party_hash",
				Action:    attSign,
			},
			{
				Name:      "verify",
				Aliases:   []string{"v"},
				Usage:     "verifies a tag and a signature",
				ArgsUsage: "message context tag signature party_hash",
				Action:    attVerify,
			},
		},
	}
	commandAuth = cli.Command{
		Name:  "auth",
		Usage: "authentication server",
		Subcommands: []cli.Command{
			{
				Name:      "store",
				Aliases:   []string{"s"},
				Usage:     "store the final statement in local configuration",
				ArgsUsage: "final.toml",
				Action:    authStore,
			},
			{
				Name:      "verify",
				Aliases:   []string{"v"},
				Usage:     "verifies a tag and a signature",
				ArgsUsage: "message context tag signature party_hash",
				Action:    attVerify,
			},
		},
	}
}
