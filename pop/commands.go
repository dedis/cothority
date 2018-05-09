package main

import "gopkg.in/urfave/cli.v1"

/*
This holds the cli-commands so the main-file is less cluttered.
*/

var commandOrg, commandAttendee, commandAuth cli.Command

func init() {

	commandOrg = cli.Command{
		Name:    "organizer",
		Aliases: []string{"org"},
		Usage:   "Organising a PoParty",
		Subcommands: []cli.Command{
			{
				Name:      "link",
				Aliases:   []string{"l"},
				Usage:     "link to a cothority",
				ArgsUsage: "IP-address:port [PIN]",
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
				Name:      "proposed",
				Aliases:   []string{"prop"},
				Usage:     "fetches proposed configs",
				ArgsUsage: "IP-address:port",
				Action:    orgProposed,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "quiet, q",
						Usage: "only return proposed toml data",
					},
				},
			},
			{
				Name:      "public",
				Aliases:   []string{"p"},
				Usage:     "stores one or more public keys during the party",
				ArgsUsage: "key1,key2,key3 party_hash",
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
		Name:    "attendee",
		Aliases: []string{"att"},
		Usage:   "attendee of a pop-party",
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
				ArgsUsage: "private_key party_hash",
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
				ArgsUsage: "message context signature tag party_hash",
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
				ArgsUsage: "message context signature tag party_hash",
				Action:    attVerify,
			},
		},
	}
}
