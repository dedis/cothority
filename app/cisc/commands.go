package main

import "gopkg.in/codegangsta/cli.v1"

/*
This holds the cli-commands so the main-file is less cluttered.
*/

var commandID, commandConfig, commandKeyvalue, commandSSH cli.Command

func init() {
	commandID = cli.Command{
		Name:  "id",
		Usage: "working on the identity",
		Subcommands: []cli.Command{
			{
				Name:      "create",
				Aliases:   []string{"cr"},
				Usage:     "start a new identity",
				ArgsUsage: "group [id-name]",
				Action:    idCreate,
			},
			{
				Name:      "connect",
				Aliases:   []string{"co"},
				Usage:     "connect to an existing identity",
				ArgsUsage: "group id [id-name]",
				Action:    idConnect,
			},
			{
				Name:    "follow",
				Aliases: []string{"f"},
				Usage:   "follow an existing identity",
				Action:  idFollow,
			},
			{
				Name:    "remove",
				Aliases: []string{"rm"},
				Usage:   "remove an identity",
				Action:  idRemove,
			},
			{
				Name:    "check",
				Aliases: []string{"ch"},
				Usage:   "check the health of the cothority",
				Action:  idCheck,
			},
		},
	}
	commandConfig = cli.Command{
		Name:  "config",
		Usage: "updating and voting on config",
		Subcommands: []cli.Command{
			{
				Name:    "propose",
				Aliases: []string{"l"},
				Usage:   "propose the new config",
				Action:  configPropose,
			},
			{
				Name:    "update",
				Aliases: []string{"u"},
				Usage:   "fetch the latest config",
				Action:  configUpdate,
			},
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "list existing config and proposed",
				Action:  configList,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "p,propose",
						Usage: "will also show proposed config",
					},
				},
			},
			{
				Name:      "vote",
				Aliases:   []string{"v"},
				Usage:     "vote on existing config",
				ArgsUsage: "[yn]",
				Action:    configVote,
			},
		},
	}
	commandKeyvalue = cli.Command{
		Name:    "keyvalue",
		Aliases: []string{"kv"},
		Usage:   "storing and retrieving key/value pairs",
		Subcommands: []cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "list all values",
				Action:  kvList,
			},
			{
				Name:      "value",
				Aliases:   []string{"v"},
				Usage:     "return the value of a key",
				ArgsUsage: "key",
				Action:    kvValue,
			},
			{
				Name:      "add",
				Aliases:   []string{"a"},
				Usage:     "add a new key/value pair",
				ArgsUsage: "key value",
				Action:    kvAdd,
			},
			{
				Name:      "del",
				Aliases:   []string{"rm"},
				Usage:     "list all values",
				ArgsUsage: "key",
				Action:    kvDel,
			},
		},
	}
	commandSSH = cli.Command{
		Name:  "ssh",
		Usage: "handling your ssh-keys",
		Subcommands: []cli.Command{
			{
				Name:    "add",
				Aliases: []string{"a"},
				Usage:   "adds a new entry to the config",
				Action:  sshAdd,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "a,alias",
						Usage: "alias to use for that entry",
					},
					cli.StringFlag{
						Name:  "u,user",
						Usage: "user for that connection",
					},
					cli.StringFlag{
						Name:  "p,port",
						Usage: "port for the connection",
					},
					cli.IntFlag{
						Name:  "sec,security",
						Usage: "how many bits for the key-creation",
						Value: 2048,
					},
				},
			},
			{
				Name:      "del",
				Aliases:   []string{"rm"},
				Usage:     "deletes an entry from the config",
				ArgsUsage: "alias_or_host",
				Action:    sshDel,
			},
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "shows all entries for this device",
				Action:  sshLs,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "a,all",
						Usage: "show entries for all devices",
					},
				},
			},
			{
				Name:    "rotate",
				Aliases: []string{"r"},
				Usage:   "renews all keys - only active once the vote passed",
				Action:  sshRotate,
			},
			{
				Name:    "sync",
				Aliases: []string{"tc"},
				Usage:   "sync config and blockchain - interactive",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "tob,toblockchain",
						Usage: "force copy of config-file to blockchain",
					},
					cli.StringFlag{
						Name:  "toc,toconfig",
						Usage: "force copy of blockchain to config-file",
					},
				},
				Action: sshSync,
			},
		},
	}
}
