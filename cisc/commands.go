package main

import "gopkg.in/urfave/cli.v1"

/*
This holds the cli-commands so the main-file is less cluttered.
*/

func getCommands() cli.Commands {
	return cli.Commands{
		{
			Name:    "link",
			Aliases: []string{"ln"},
			Usage:   "create and use links with admin privileges",
			Subcommands: cli.Commands{
				{
					Name:      "pin",
					Aliases:   []string{"p"},
					Usage:     "links using a pin written to the log-file on the conode",
					ArgsUsage: "ip:port [PIN]",
					Action:    linkPin,
				},
				{
					Name:      "addfinal",
					Aliases:   []string{"af"},
					Usage:     "adds a final statement to a linked remote node for use by attendees",
					ArgsUsage: "final_statement.toml ip:port",
					Action:    linkAddFinal,
				},
				{
					Name:      "addpublic",
					Aliases:   []string{"ap"},
					Usage:     "adds a public key to a linked remote node for use by attendees",
					ArgsUsage: "public_key ip:port",
					Action:    linkAddPublic,
				},
				{
					Name:    "keypair",
					Aliases: []string{"kp"},
					Usage:   "create a keypair for usage in private/public key",
					Action:  linkPair,
				},
				{
					Name:    "list",
					Aliases: []string{"ls", "l"},
					Usage:   "show a list of all links stored on this client",
					Action:  linkList,
				},
			},
		},

		{
			Name:    "skipchain",
			Aliases: []string{"sc"},
			Usage:   "work with the underlying skipchain",
			Subcommands: []cli.Command{
				{
					Name:      "create",
					Aliases:   []string{"cr", "c"},
					Usage:     "start a new identity",
					ArgsUsage: "group.toml",
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "threshold, thr",
							Usage: "the threshold necessary to add a block",
							Value: 2,
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of this device in the cisc",
						},
						cli.StringFlag{
							Name:  "private, priv",
							Usage: "give the private key to authenticate",
						},
						cli.StringFlag{
							Name:  "token, tok",
							Usage: "give a pop-token to authenticate",
						},
					},
					Action: scCreate,
				},
				{
					Name:      "join",
					Aliases:   []string{"j"},
					Usage:     "propose to join an existing identity",
					ArgsUsage: "group.toml id",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the device used in the identity",
						},
					},
					Action: scJoin,
				},
				{
					Name:      "del",
					Aliases:   []string{"d", "rm"},
					Usage:     "remove this device from an identity",
					ArgsUsage: "name [skipchain-id]",
					Action:    scDel,
				},
				{
					Name:    "list",
					Aliases: []string{"ls", "l"},
					Usage:   "show all stored skipchains",
					Action:  scList,
				},
				{
					Name:      "qrcode",
					Aliases:   []string{"qr"},
					Usage:     "print out the qrcode of the identity-skipchain and a node for contact",
					ArgsUsage: "[skipchain-id]",
					Action:    scQrcode,
				},
				{
					Name:      "roster",
					Aliases:   []string{"r"},
					Usage:     "define a new roster for the skipchain",
					ArgsUsage: "group.toml [skipchain-id]",
					Action:    scRoster,
				},
			},
		},

		{
			Name:    "data",
			Aliases: []string{"cfg"},
			Usage:   "updating and voting on data",
			Subcommands: []cli.Command{
				{
					Name:      "clear",
					Aliases:   []string{"c"},
					Usage:     "clear the proposition",
					ArgsUsage: "[skipchain-id]",
					Action:    dataClear,
				},
				{
					Name:      "update",
					Aliases:   []string{"upd"},
					Usage:     "fetch the latest data",
					ArgsUsage: "[skipchain-id]",
					Action:    dataUpdate,
				},
				{
					Name:    "list",
					Aliases: []string{"ls", "l"},
					Usage:   "list existing data and proposed",
					Action:  dataList,
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "propose, p",
							Usage: "will also show proposed data",
						},
						cli.BoolFlag{
							Name:  "details, d",
							Usage: "also show the values of the keys",
						},
					},
				},
				{
					Name:      "vote",
					Aliases:   []string{"v"},
					Usage:     "vote on proposed data",
					ArgsUsage: "[skipchain-id]",
					Action:    dataVote,
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "no, n",
							Usage: "refuse vote",
						},
						cli.BoolFlag{
							Name:  "yes, y",
							Usage: "accept vote",
						},
					},
				},
			},
		},

		{
			Name:    "keyvalue",
			Aliases: []string{"kv"},
			Usage:   "storing and retrieving key/value pairs",
			Subcommands: []cli.Command{
				{
					Name:    "list",
					Aliases: []string{"ls", "l"},
					Usage:   "list all values",
					Action:  kvList,
				},
				{
					Name:      "value",
					Aliases:   []string{"v"},
					Usage:     "return the value of a key",
					ArgsUsage: "key [skipchain-id]",
					Action:    kvValue,
				},
				{
					Name:      "add",
					Aliases:   []string{"a"},
					Usage:     "add a new key/value pair",
					ArgsUsage: "key value [skipchain-id]",
					Action:    kvAdd,
				},
				{
					Name:      "del",
					Aliases:   []string{"d", "rm"},
					Usage:     "delete a value",
					ArgsUsage: "key [skipchain-id]",
					Action:    kvDel,
				},
			},
		},

		{
			Name:  "ssh",
			Usage: "interacting with the ssh-keys stored in the skipchain",
			Subcommands: []cli.Command{
				{
					Name:      "add",
					Aliases:   []string{"a"},
					Usage:     "adds a new entry to the skipchain",
					Action:    sshAdd,
					ArgsUsage: "hostname [skipchain-id]",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "alias, a",
							Usage: "alias to use for that entry",
						},
						cli.StringFlag{
							Name:  "user, u",
							Usage: "user for that connection",
						},
						cli.StringFlag{
							Name:  "port, p",
							Usage: "port for the connection",
						},
						cli.IntFlag{
							Name:  "security, sec",
							Usage: "how many bits for the key-creation",
							Value: 2048,
						},
					},
				},
				{
					Name:      "del",
					Aliases:   []string{"d", "rm"},
					Usage:     "deletes an entry from the skipchain",
					ArgsUsage: "alias_or_host [skipchain-id]",
					Action:    sshDel,
				},
				{
					Name:      "list",
					Aliases:   []string{"ls"},
					Usage:     "shows all entries for this device",
					Action:    sshLs,
					ArgsUsage: "[skipchain-id]",
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
					Aliases: []string{"s"},
					Usage:   "sync ssh-config and blockchain - interactive",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "toblockchain, tob",
							Usage: "force copy of ssh-config-file to blockchain",
						},
						cli.StringFlag{
							Name:  "toconfig, toc",
							Usage: "force copy of blockchain to ssh-config-file",
						},
					},
					Action: sshSync,
				},
			},
		},

		{
			Name:    "follow",
			Aliases: []string{"f"},
			Usage:   "follow skipchains",
			Subcommands: []cli.Command{
				{
					Name:      "add",
					Aliases:   []string{"a"},
					Usage:     "add a new skipchain",
					ArgsUsage: "group ID service-name",
					Action:    followAdd,
				},
				{
					Name:      "del",
					Aliases:   []string{"d", "rm"},
					Usage:     "delete a skipchain",
					ArgsUsage: "ID",
					Action:    followDel,
				},
				{
					Name:    "list",
					Aliases: []string{"ls", "l"},
					Usage:   "list all skipchains and keys",
					Action:  followList,
				},
				{
					Name:    "update",
					Aliases: []string{"upd"},
					Usage:   "update all skipchains",
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "poll, p",
							Value: 0,
							Usage: "poll every n seconds",
						},
					},
					Action: followUpdate,
				},
			},
		},

		{
			Name:      "web",
			Usage:     "add a web-site to a skipchain",
			Aliases:   []string{"w"},
			ArgsUsage: "path/page.html [skipchain-id]",
			Action:    kvAddWeb,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "inline",
					Usage: "inline all images, css and scripts",
				},
			},
		},
	}
}
