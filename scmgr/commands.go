package main

import cli "github.com/urfave/cli"

func getCommands() cli.Commands {
	groupsDef := "[group-definition]"
	return cli.Commands{
		{
			Name:    "link",
			Usage:   "create a secure link to a conode",
			Aliases: []string{"l"},
			Subcommands: cli.Commands{
				{
					Name:      "add",
					Usage:     "create a secure link to a conode",
					ArgsUsage: "private.toml",
					Aliases:   []string{"a"},
					Action:    linkAdd,
				},
				{
					Name:      "del",
					Usage:     "remove a secure link to a conode",
					ArgsUsage: "ip:port",
					Aliases:   []string{"d", "rm"},
					Action:    linkDel,
				},
				{
					Name:    "list",
					Usage:   "show all secure links stored in scmgr",
					Aliases: []string{"ls"},
					Action:  linkList,
				},
				{
					Name:      "query",
					Usage:     "request list of secure links in conode",
					ArgsUsage: "ip:port",
					Aliases:   []string{"q"},
					Action:    linkQuery,
				},
			},
		},

		{
			Name:    "follow",
			Usage:   "allow conode to be included in skipchain",
			Aliases: []string{"f"},
			Subcommands: cli.Commands{
				{
					Name:      "add",
					Usage:     "allow inclusion of conode in skipchain",
					ArgsUsage: "skipchain-id",
					Aliases:   []string{"a"},
					Subcommands: cli.Commands{
						{
							Name:      "single",
							Usage:     "only allow inclusion in this specific chain",
							ArgsUsage: "skipchain-id ip:port",
							Action:    followAddID,
						},
						{
							Name:      "roster",
							Usage:     "allow inclusion of conode in new skipchains from roster of specified skipchain",
							ArgsUsage: "skipchain-id ip:port",
							Action:    followAddRoster,
							Flags: []cli.Flag{
								cli.StringFlag{
									Name:  "lookup, l",
									Usage: "IP:Port - conode that has a copy of the skipchain",
								},
								cli.BoolFlag{
									Name:  "any, a",
									Usage: "Allow new chain if any of the nodes is present",
								},
							},
						},
					},
				},
				{
					Name:      "delete",
					Usage:     "remove authorization for inclusion in skipchain",
					Aliases:   []string{"del", "rm", "d"},
					ArgsUsage: "skipchain-id ip:port",
					Action:    followDel,
				},
				{
					Name:      "list",
					Usage:     "list all skipchains a conode follows",
					ArgsUsage: "ip:port",
					Aliases:   []string{"ls"},
					Action:    followList,
				},
			},
		},

		{
			Name:    "skipchain",
			Usage:   "work with skipchains in cothority",
			Aliases: []string{"s"},
			Subcommands: cli.Commands{
				{
					Name:      "create",
					Usage:     "make a new skipchain",
					Aliases:   []string{"c"},
					ArgsUsage: groupsDef,
					Action:    scCreate,
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "base, b",
							Value: 2,
							Usage: "base for skipchains",
						},
						cli.IntFlag{
							Name:  "height, he",
							Value: 2,
							Usage: "maximum height of skipchain",
						},
					},
				},
				{
					Name:    "block",
					Usage:   "work on blocks of an existing skipchain",
					Aliases: []string{"b"},
					Subcommands: cli.Commands{
						{
							Name:      "add",
							Usage:     "create a new block on the server and save it in local cache",
							Aliases:   []string{"a"},
							ArgsUsage: "skipchain-id",
							Action:    scAdd,
							Flags: []cli.Flag{
								cli.StringFlag{
									Name:  "roster, r",
									Value: "",
									Usage: "file containing new roster",
								},
								cli.StringFlag{
									Name:  "data",
									Value: "",
									Usage: "data to put into the block",
								},
							},
						},
						{
							Name:      "print",
							Usage:     "show the content of a block from the local cache",
							Aliases:   []string{"p"},
							ArgsUsage: "skipblock-id",
							Action:    scPrint,
						},
					},
				},
				{
					Name:      "updates",
					Usage:     "get the list of updated blocks for a chain",
					Aliases:   []string{"u"},
					ArgsUsage: groupsDef,
					Action:    scUpdates,
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "from",
							Usage: "last known block hash",
						},
						cli.IntFlag{
							Name:  "level",
							Value: -1,
							Usage: "maximum height links to use (default: longest forward links available)",
						},
						cli.IntFlag{
							Name:  "count",
							Value: -1,
							Usage: "limit on how many blocks to fetch (default: all available blocks)",
						},
					},
				},
				{
					Name:    "optimize",
					Usage:   "create missing forward link to optimize the proof of a given block",
					Aliases: []string{"o"},
					Action:  scOptimize,
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "roster",
							Usage: "conodes where to propagate",
						},
						cli.StringFlag{
							Name:  "id",
							Usage: "target block or skipchain to optimize",
						},
					},
				},
			},
		},

		{
			Name:    "scdns",
			Usage:   "skipchain dns handling for web frontend",
			Aliases: []string{"d"},
			Subcommands: []cli.Command{
				{
					Name:      "fetch",
					Usage:     "request a skipchain and store it locally",
					Aliases:   []string{"f"},
					ArgsUsage: groupsDef + " skipchain-id",
					Action:    dnsFetch,
				},
				{
					Name:    "list",
					Aliases: []string{"l"},
					Usage:   "lists all locally stored skipchains",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "long, l",
							Usage: "give long id of blocks",
						},
					},
					Action: dnsList,
				},
				{
					Name:      "index",
					Aliases:   []string{"i"},
					Usage:     "create index-files for all known skipchains",
					ArgsUsage: "output_path",
					Action:    dnsIndex,
				},
				{
					Name:    "update",
					Aliases: []string{"u"},
					Usage:   "update all locally stored skipchains",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "new, n",
							Usage: "fetch new skipchains from conodes",
						},
						cli.StringFlag{
							Name:  "roster, r",
							Usage: "add skipchains from conodes of this roster",
						},
					},
					Action: dnsUpdate,
				},
			},
		},
	}
}
