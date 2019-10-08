package main

import (
	"fmt"
	"time"

	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/clicontracts"
)

var cmds = cli.Commands{
	{
		Name:      "create",
		Usage:     "create a ledger",
		Aliases:   []string{"c"},
		ArgsUsage: "[roster.toml]",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "roster, r",
				Usage: "the roster of the cothority that will host the ledger",
			},
			cli.DurationFlag{
				Name:  "interval, i",
				Usage: "the block interval for this ledger",
				Value: 5 * time.Second,
			},
		},
		Action: create,
	},

	{
		Name:        "link",
		Usage:       "create a BC config file that sets the specified roster, darc and identity",
		Description: "If no identity is provided, it will use an empty one. Same for the darc param. This allows one that has no private key to perform basic operations that do not require authentication.",
		Aliases:     []string{"login"},
		ArgsUsage:   "roster.toml [byzcoin id]",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "darc",
				Usage: "the darc id to be saved (defaults to an empty darc)",
			},
			cli.StringFlag{
				Name:  "identity, id",
				Usage: "the identity to be saved (defaults to an empty identity)",
			},
		},
		Action: link,
	},

	{
		Name:      "latest",
		Usage:     "show the latest block in the chain",
		Aliases:   []string{"s"},
		ArgsUsage: "[bc.cfg]",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use",
			},
			cli.IntFlag{
				Name:  "server",
				Usage: "which server number from the roster to contact (default: -1 = random)",
				Value: -1,
			},
			cli.BoolFlag{
				Name:  "update",
				Usage: "update the ByzCoin config file with the fetched roster",
			},
			cli.BoolFlag{
				Name:  "roster",
				Usage: "display the latest block's roster",
			},
			cli.BoolFlag{
				Name:  "header",
				Usage: "display the latest header",
			},
		},
		Action: latest,
	},

	{
		Name:      "db",
		Usage:     "interact with byzcoin for debugging",
		Aliases:   []string{"d"},
		ArgsUsage: "conode.db [byzCoinID]",
		Subcommands: cli.Commands{
			{
				Name:   "status",
				Usage:  "returns the status of the db",
				Action: dbStatus,
			},
			{
				Name:      "catchup",
				Usage:     "Fetch new blocks from an active chain",
				Action:    dbCatchup,
				ArgsUsage: "URL",
				Flags: []cli.Flag{
					cli.IntFlag{
						Name: "batch",
						Usage: "how many blocks will be fetched with each" +
							" request",
						Value: 100,
					},
				},
			},
			{
				Name:   "replay",
				Usage:  "Replay a chain and check the global state is consistent",
				Action: dbReplay,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "continue, cont",
						Usage: "continue an aborted replay",
					},
					cli.IntFlag{
						Name:  "blocks",
						Usage: "how many blocks to apply",
					},
				},
			},
			{
				Name:      "merge",
				Usage:     "Copy the blocks of another db-file into this one",
				Action:    dbMerge,
				ArgsUsage: "conode2.db",
				Flags: []cli.Flag{
					cli.IntFlag{
						Name:  "blocks",
						Usage: "maximum number of blocks to merge",
					},
					cli.BoolFlag{
						Name:  "overwrite",
						Usage: "replace whole blocks if they are duplicate",
					},
				},
			},
			{
				Name: "check",
				Usage: "Check that the chain is in a correct state with" +
					" regard to hashes, forward-, and backward-links",
				Action: dbCheck,
				Flags: []cli.Flag{
					cli.IntFlag{
						Name:  "blocks",
						Usage: "maximum number of blocks to check",
					},
					cli.IntFlag{
						Name:  "start",
						Usage: "index of block to start verifications",
					},
					cli.IntFlag{
						Name:  "process",
						Usage: "show process indicator every n blocks",
						Value: 100,
					},
				},
			},
		},
	},

	{
		Name:    "debug",
		Usage:   "interact with byzcoin for debugging",
		Aliases: []string{"d"},
		Subcommands: cli.Commands{
			{
				Name:   "block",
				Usage:  "Read a block given by an id or an index",
				Action: debugBlock,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name: "bcCfg, bc",
						Usage: "pass a byzcoin-config to define the bcID and" +
							" the roster",
					},
					cli.StringFlag{
						Name:  "url",
						Usage: "pass a url to indicate the node to contact",
					},
					cli.StringFlag{
						Name:  "bcID",
						Usage: "give byzcoin-ID to search for",
					},
					cli.BoolFlag{
						Name:  "all",
						Usage: "show block in all nodes",
					},
					cli.IntFlag{
						Name:  "blockIndex, index",
						Value: -1,
						Usage: "give this block-index",
					},
					cli.StringFlag{
						Name:  "blockID, id",
						Usage: "give block-id to show",
					},
					cli.BoolFlag{
						Name:  "txDetails, txd",
						Usage: "prints all transactions",
					},
				},
			},
			{
				Name:   "list",
				Usage:  "Lists all byzcoin instances",
				Action: debugList,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "verbose, v",
						Usage: "print more information of the instances",
					},
				},
				ArgsUsage: "(ip:port | group.toml)",
			},
			{
				Name:  "dump",
				Usage: "dumps a given byzcoin instance",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "verbose, v",
						Usage: "print more information of the instances",
					},
				},
				Action:    debugDump,
				ArgsUsage: "ip:port byzcoin-id",
			},
			{
				Name:      "remove",
				Usage:     "removes a given byzcoin instance",
				Action:    debugRemove,
				ArgsUsage: "private.toml byzcoin-id",
			},
			{
				Name:      "counters",
				Usage:     "shows the counter-state in all nodes",
				Action:    debugCounters,
				ArgsUsage: "bc.cfg key-file",
			},
		},
	},

	{
		Name:      "mint",
		Usage:     "mint coins on account",
		ArgsUsage: "bc-xxx.cfg key-xxx.cfg public-key #coins",
		Action:    mint,
	},

	{
		Name:    "roster",
		Usage:   "change the roster of the ByzCoin",
		Aliases: []string{"r"},
		Subcommands: cli.Commands{
			{
				Name:      "add",
				ArgsUsage: "bc-xxx.cfg key-xxx.cfg public.toml",
				Usage:     "Add a new node to the roster",
				Action:    rosterAdd,
			},
			{
				Name:      "del",
				ArgsUsage: "bc-xxx.cfg key-xxx.cfg public.toml",
				Usage:     "Remove a node from the roster",
				Action:    rosterDel,
			},
			{
				Name:      "leader",
				ArgsUsage: "bc-xxx.cfg key-xxx.cfg public.toml",
				Usage:     "Set a specific node to be the leader",
				Action:    rosterLeader,
			},
		},
	},

	{
		Name:      "config",
		Usage:     "update the config",
		ArgsUsage: "bc-xxx.cfg key-xxx.cfg",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "interval",
				Usage: "change the interval",
			},
			cli.IntFlag{
				Name:  "blockSize",
				Usage: "adjust the maximum block size",
			},
		},
		Action: config,
	},

	{
		Name:    "key",
		Usage:   "generates a new keypair and prints the public key in the stdout",
		Aliases: []string{"k"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "save",
				Usage: "file in which the user wants to save the public key instead of printing it",
			},
			cli.StringFlag{
				Name:  "print",
				Usage: "print the private and public key",
			},
		},
		Action: key,
	},

	{
		Name:    "darc",
		Usage:   "tool used to manage darcs",
		Aliases: []string{"d"},
		Subcommands: cli.Commands{
			{
				Name:   "show",
				Usage:  "Show a DARC",
				Action: darcShow,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:   "bc",
						EnvVar: "BC",
						Usage:  "the ByzCoin config to use (required)",
					},
					cli.StringFlag{
						Name:  "darc",
						Usage: "the darc to show (admin darc by default)",
					},
				},
			},
			{
				Name:   "cdesc",
				Usage:  "Edit the description of a DARC",
				Action: darcCdesc,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:   "bc",
						EnvVar: "BC",
						Usage:  "the ByzCoin config to use (required)",
					},
					cli.StringFlag{
						Name:  "darc",
						Usage: "the id of the darc to edit (config admin darc by default)",
					},
					cli.StringFlag{
						Name:  "desc",
						Usage: "the new description of the darc (required)",
					},
				},
			},
			{
				Name:   "add",
				Usage:  "Add a new DARC with default rules.",
				Action: darcAdd,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:   "bc",
						EnvVar: "BC",
						Usage:  "the ByzCoin config to use (required)",
					},
					cli.StringFlag{
						Name:  "sign, signer",
						Usage: "public key which will sign the DARC spawn request (default: the ledger admin identity)",
					},
					cli.StringFlag{
						Name:  "darc",
						Usage: "DARC with the right to create a new DARC (default is the admin DARC)",
					},
					cli.StringSliceFlag{
						Name:  "identity, id",
						Usage: "an identity, multiple use of this param is allowed. If empty it will create a new identity. Each provided identity is checked by the evaluation parser.",
					},
					cli.BoolFlag{
						Name:  "unrestricted",
						Usage: "add the invoke:evolve_unrestricted rule",
					},
					cli.BoolFlag{
						Name:  "deferred",
						Usage: "adds rules related to deferred contract: spawn:deferred, invoke:deferred.addProof, invoke:deferred.execProposedTx",
					},
					cli.StringFlag{
						Name:  "out_id",
						Usage: "output file for the darc id (optional)",
					},
					cli.StringFlag{
						Name:  "out_key",
						Usage: "output file for the darc key (optional)",
					},
					cli.StringFlag{
						Name:  "desc",
						Usage: "the description for the new DARC (default: random)",
					},
				},
			},
			{
				Name:   "prule",
				Usage:  "print rule. Will print the rule given identities and a minimum to have M out of N rule",
				Action: darcPrintRule,
				Flags: []cli.Flag{
					cli.StringSliceFlag{
						Name:  "identity, id",
						Usage: "an identity, multiple use of this param is allowed. If empty it will create a new identity. Each provided identity is checked by the evaluation parser.",
					},
					cli.UintFlag{
						Name:  "minimum, M",
						Usage: "if this flag is set, the rule is computed to be \"M out of N\" identities. Otherwise it uses ANDs",
					},
				},
			},
			{
				Name:   "rule",
				Usage:  "Edit DARC rules.",
				Action: darcRule,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:   "bc",
						EnvVar: "BC",
						Usage:  "the ByzCoin config to use (required)",
					},
					cli.StringFlag{
						Name:  "darc",
						Usage: "the DARC to update (default is the admin DARC)",
					},
					cli.StringFlag{
						Name:  "sign",
						Usage: "public key of the signing entity (default is the admin public key)",
					},
					cli.StringFlag{
						Name:  "rule",
						Usage: "the rule to be added, updated or deleted",
					},
					cli.StringSliceFlag{
						Name:  "identity, id",
						Usage: "the identity of the signer who will be allowed to use the rule. Multiple use of this param is allowed. Each identity is checked by the evaluation parser.",
					},
					cli.UintFlag{
						Name:  "minimum, M",
						Usage: "if this flag is set, the rule is computed to be \"M out of N\" identities. Otherwise it uses ANDs",
					},
					cli.BoolFlag{
						Name:  "replace",
						Usage: "if this rule already exists, replace it with this new one",
					},
					cli.BoolFlag{
						Name:  "delete",
						Usage: "delete the rule",
					},
				},
			},
		},
	},

	{
		Name:    "qr",
		Usage:   "generates a QRCode containing the description of the BC Config",
		Aliases: []string{"qrcode"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use (required)",
			},
			cli.BoolFlag{
				Name:  "admin",
				Usage: "If specified, the QR Code will contain the admin keypair",
			},
		},
		Action: qrcode,
	},

	{
		Name:  "info",
		Usage: "displays infos about the BC config",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use (required)",
			},
		},
		Action: getInfo,
	},

	{
		Name:  "resolveiid",
		Usage: "Resolves an instance id given a name and a darc id (using the ResolveInstanceID API call)",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use (required)",
			},
			cli.StringFlag{
				Name:  "namingDarc",
				Usage: "the DARC ID that 'guards' the instance (default is the admin darc)",
			},
			cli.StringFlag{
				Name:  "name",
				Usage: "the name that was used to store the instance id (required)",
			},
		},
		Action: resolveiid,
	},

	{
		Name: "contract",
		// Use space instead of tabs for correct formatting
		Usage: "Provides cli interface for contracts",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "export, x",
				Usage: "redirects the transaction to stdout",
			},
		},
		// UsageText should be used instead, but its not working:
		// see https://github.com/urfave/cli/issues/592
		Description: fmt.Sprint(`
   bcadmin [--export] contract CONTRACT { 
                               spawn  --bc <byzcoin config> 
                                      [--<arg name> <arg value>, ...]
                                      [--darc <darc id>] 
                                      [--sign <pub key>],
                               invoke <command>
                                      --bc <byzcoin config>
                                      --instid, i <instance ID>
                                      [--<arg name> <arg value>, ...]
                                      [--darc <darc id>] 
                                      [--sign <pub key>],
                               get    --bc <byzcoin config>
                                      --instid, i <instance ID>,
                               delete --bc <byzcoin config>
                                      --instid, i <instance ID>
                                      [--darc <darc id>] 
                                      [--sign <pub key>]     
                             }
   CONTRACT   {value,deferred,config}`),
		Subcommands: cli.Commands{
			{
				Name:  "value",
				Usage: "Manipulate a value contract",
				Subcommands: cli.Commands{
					{
						Name:   "spawn",
						Usage:  "spawn a value contract",
						Action: clicontracts.ValueSpawn,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
							cli.StringFlag{
								Name:  "value",
								Usage: "the value to save",
							},
							cli.StringFlag{
								Name:  "darc",
								Usage: "DARC with the right to spawn a value contract (default is the admin DARC)",
							},
							cli.StringFlag{
								Name:  "sign",
								Usage: "public key of the signing entity (default is the admin public key)",
							},
						},
					},
					{
						Name:  "invoke",
						Usage: "invoke a value contract",
						Subcommands: cli.Commands{
							{
								Name:   "update",
								Usage:  "update the value of a value contract",
								Action: clicontracts.ValueInvokeUpdate,
								Flags: []cli.Flag{
									cli.StringFlag{
										Name:   "bc",
										EnvVar: "BC",
										Usage:  "the ByzCoin config to use (required)",
									},
									cli.StringFlag{
										Name:  "value",
										Usage: "the value to save",
									},
									cli.StringFlag{
										Name:  "instid, i",
										Usage: "the instance ID of the value contract",
									},
									cli.StringFlag{
										Name:  "darc",
										Usage: "DARC with the right to invoke.update a value contract (default is the admin DARC)",
									},
									cli.StringFlag{
										Name:  "sign",
										Usage: "public key of the signing entity (default is the admin public key)",
									},
								},
							},
						},
					},
					{
						Name:   "get",
						Usage:  "if the proof matches, get the content of the given value instance ID",
						Action: clicontracts.ValueGet,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
							cli.StringFlag{
								Name:  "instid, i",
								Usage: "the instance id (required)",
							},
						},
					},

					{
						Name:   "delete",
						Usage:  "delete a value contract",
						Action: clicontracts.ValueDelete,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
							cli.StringFlag{
								Name:  "instid, i",
								Usage: "the instance ID of the value contract",
							},
							cli.StringFlag{
								Name:  "darc",
								Usage: "DARC with the right to invoke.update a value contract (default is the admin DARC)",
							},
							cli.StringFlag{
								Name:  "sign",
								Usage: "public key of the signing entity (default is the admin public key)",
							},
						},
					},
				},
			},
			{
				Name:  "deferred",
				Usage: "Manipulate a deferred contract",
				Subcommands: cli.Commands{
					{
						Name:   "spawn",
						Usage:  "spawn a deferred contract with the proposed transaction in stdin",
						Action: clicontracts.DeferredSpawn,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
							cli.StringFlag{
								Name:  "darc",
								Usage: "DARC with the right to spawn a deferred contract (default is the admin DARC)",
							},
							cli.StringFlag{
								Name:  "sign",
								Usage: "public key of the signing entity (default is the admin public key)",
							},
						},
					},
					{
						Name:  "invoke",
						Usage: "invoke on a deferred contract ",
						Subcommands: cli.Commands{
							{
								Name:   "addProof",
								Usage:  "adds a signature and an identity on an instruction of the proposed transaction",
								Action: clicontracts.DeferredInvokeAddProof,
								Flags: []cli.Flag{
									cli.StringFlag{
										Name:   "bc",
										EnvVar: "BC",
										Usage:  "the ByzCoin config to use (required)",
									},
									cli.UintFlag{
										Name:  "instrIdx",
										Usage: "the instruction index of the transaction (starts from 0) (default is 0)",
									},
									cli.StringFlag{
										Name:  "hash",
										Usage: "the instruction hash that will be signed",
									},
									cli.StringFlag{
										Name:  "instid, i",
										Usage: "the instance ID of the deferred contract",
									},
									cli.StringFlag{
										Name:  "darc",
										Usage: "DARC with the right to invoke.addProof a deferred contract (default is the admin DARC)",
									},
									cli.StringFlag{
										Name:  "sign",
										Usage: "public key of the signing entity (default is the admin public key)",
									},
								},
							},
							{
								Name:   "execProposedTx",
								Usage:  "executes the proposed transaction if the instructions are correctly signed",
								Action: clicontracts.ExecProposedTx,
								Flags: []cli.Flag{
									cli.StringFlag{
										Name:   "bc",
										EnvVar: "BC",
										Usage:  "the ByzCoin config to use (required)",
									},
									cli.StringFlag{
										Name:  "instid, i",
										Usage: "the instance ID of the deferred contract",
									},
									cli.StringFlag{
										Name:  "darc",
										Usage: "DARC with the right to invoke.execProposedTx a deferred contract (default is the admin DARC)",
									},
									cli.StringFlag{
										Name:  "sign",
										Usage: "public key of the signing entity (default is the admin public key)",
									},
								},
							},
						},
					},
					{
						Name:   "get",
						Usage:  "if the proof matches, get the content of the given deferred instance ID",
						Action: clicontracts.DeferredGet,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
							cli.StringFlag{
								Name:  "instid, i",
								Usage: "the instance id (required)",
							},
						},
					},

					{
						Name:   "delete",
						Usage:  "delete a deferred contract",
						Action: clicontracts.DeferredDelete,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
							cli.StringFlag{
								Name:  "instid, i",
								Usage: "the instance ID of the value contract",
							},
							cli.StringFlag{
								Name:  "darc",
								Usage: "DARC with the right to invoke.update a value contract (default is the admin DARC)",
							},
							cli.StringFlag{
								Name:  "sign",
								Usage: "public key of the signing entity (default is the admin public key)",
							},
						},
					},
				},
			},
			{
				Name:  "config",
				Usage: "Manipulate a config contract",
				Subcommands: cli.Commands{
					{
						Name:  "invoke",
						Usage: "invoke on a config contract ",
						Subcommands: cli.Commands{
							{
								Name:   "updateConfig",
								Usage:  "changes the roster's leader",
								Action: clicontracts.ConfigInvokeUpdateConfig,
								Flags: []cli.Flag{
									cli.StringFlag{
										Name:   "bc",
										EnvVar: "BC",
										Usage:  "the ByzCoin config to use (required)",
									},
									cli.StringFlag{
										Name:  "sign",
										Usage: "public key of the signing entity (default is the admin public key)",
									},
									cli.StringFlag{
										Name:  "blockInterval",
										Usage: "blockInterval, for example 2s (optional)",
									},
									cli.IntFlag{
										Name:  "maxBlockSize",
										Usage: "maxBlockSize (optional)",
									},
									cli.StringFlag{
										Name:  "darcContractIDs",
										Usage: "darcContractIDs separated by comas (optional)",
									},
								},
							},
						},
					},
					{
						Name:   "get",
						Usage:  "displays the latest chain config",
						Action: clicontracts.ConfigGet,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
						},
					},
				},
			},

			{
				Name:  "name",
				Usage: "Manipulate the name contract",
				Subcommands: cli.Commands{
					{
						Name:  "invoke",
						Usage: "invoke on a config contract",
						Subcommands: cli.Commands{
							{
								Name:   "add",
								Usage:  "add a name resolver",
								Action: clicontracts.NameInvokeAdd,
								Flags: []cli.Flag{
									cli.StringFlag{
										Name:   "bc",
										EnvVar: "BC",
										Usage:  "the ByzCoin config to use (required)",
									},
									cli.StringFlag{
										Name:  "sign",
										Usage: "public key of the signing entity (default is the admin public key)",
									},
									cli.StringFlag{
										Name:  "name",
										Usage: "the name to use",
									},
									cli.StringSliceFlag{
										Name: "instid, i",
										Usage: "the instance id to 'save'. Its darc must have the _name rule on it and this parameter can be used multiple times. " +
											"If used mulitple times (more than once), a random generated string is appended to the name in order to avoid conflicts.",
									},
									cli.BoolFlag{
										Name:  "append, a",
										Usage: "even if only one instance id is provided, appends a random string to the name",
									},
								},
							},
							{
								Name:   "remove",
								Usage:  "remove a name resolver",
								Action: clicontracts.NameInvokeRemove,
								Flags: []cli.Flag{
									cli.StringFlag{
										Name:   "bc",
										EnvVar: "BC",
										Usage:  "the ByzCoin config to use (required)",
									},
									cli.StringFlag{
										Name:  "sign",
										Usage: "public key of the signing entity (default is the admin public key)",
									},
									cli.StringFlag{
										Name:  "name",
										Usage: "the name to delete",
									},
									cli.StringFlag{
										Name:  "instid, i",
										Usage: "the instance id the name is pointing to. Used to get its darc.",
									},
								},
							},
						},
					},
					{
						Name:   "spawn",
						Usage:  "spawn the name contract (can only be done once)",
						Action: clicontracts.NameSpawn,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
							cli.StringFlag{
								Name:  "sign",
								Usage: "public key of the signing entity (default is the admin public key)",
							},
						},
					},
					{
						Name:   "get",
						Usage:  "displays the name instance",
						Action: clicontracts.NameGet,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
						},
					},
				},
			},
		},
	},
}
