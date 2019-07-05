package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/qantik/qrgo"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/clicontracts"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	_ "go.dedis.ch/cothority/v3/personhood"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	cli "gopkg.in/urfave/cli.v1"
)

type chainFetcher func(si *network.ServerIdentity) ([]skipchain.SkipBlockID, error)

const errUnregisteredMessage = "The requested message hasn't been registered"

func init() {
	network.RegisterMessages(&darc.Darc{}, &darc.Identity{}, &darc.Signer{})
}

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
				Usage: "which server number from the roster to contact (default: 0)",
				Value: 0,
			},
			cli.BoolFlag{
				Name:  "update",
				Usage: "update the ByzCoin config file with the fetched roster",
			},
			cli.BoolFlag{
				Name:  "roster",
				Usage: "display the latest block's roster",
			},
		},
		Action: latest,
	},

	{
		Name:    "debug",
		Usage:   "interact with byzcoin for debugging",
		Aliases: []string{"d"},
		Subcommands: cli.Commands{
			{
				Name:      "replay",
				Usage:     "Replay a chain and check the global state is consistent",
				Action:    debugReplay,
				ArgsUsage: "URL",
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
   CONTRAT   {value,deferred,config}`),
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
		},
	},
}

var cliApp = cli.NewApp()

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

var gitTag = "dev"

func init() {
	cliApp.Name = "bcadmin"
	cliApp.Usage = "Create ByzCoin ledgers and grant access to them."
	cliApp.Version = gitTag
	cliApp.Commands = cmds
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:   "config, c",
			EnvVar: "BC_CONFIG",
			Value:  getDataPath(cliApp.Name),
			Usage:  "path to configuration-directory",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		lib.ConfigPath = c.String("config")
		return nil
	}
}

func main() {
	rand.Seed(time.Now().Unix())
	err := cliApp.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func create(c *cli.Context) error {
	fn := c.String("roster")
	if fn == "" {
		fn = c.Args().First()
		if fn == "" {
			return errors.New("roster argument or --roster flag is required")
		}
	}
	r, err := lib.ReadRoster(fn)
	if err != nil {
		return err
	}

	interval := c.Duration("interval")

	owner := darc.NewSignerEd25519(nil, nil)

	req, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, r, []string{"spawn:longTermSecret"}, owner.Identity())
	if err != nil {
		log.Error(err)
		return err
	}
	req.BlockInterval = interval

	_, resp, err := byzcoin.NewLedger(req, false)
	if err != nil {
		return err
	}

	cfg := lib.Config{
		ByzCoinID:     resp.Skipblock.SkipChainID(),
		Roster:        *r,
		AdminDarc:     req.GenesisDarc,
		AdminIdentity: owner.Identity(),
	}
	fn, err = lib.SaveConfig(cfg)
	if err != nil {
		return err
	}

	err = lib.SaveKey(owner)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.App.Writer, "Created ByzCoin with ID %x.\n", cfg.ByzCoinID)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.App.Writer, "export BC=\"%v\"\n", fn)
	if err != nil {
		return err
	}

	// For the tests to use.
	c.App.Metadata["BC"] = fn

	return nil
}

func link(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("please give the following args: roster.toml [byzcoin id]")
	}
	r, err := lib.ReadRoster(c.Args().First())
	if err != nil {
		return err
	}

	if c.NArg() == 1 {
		log.Info("Fetching all byzcoin-ids from the roster")
		var scIDs []skipchain.SkipBlockID
		for _, si := range r.List {
			ids, err := fetchChains(si, byzcoinFetcher, skipchainFetcher)
			if err != nil {
				log.Warn("Couldn't contact", si.Address, err)
			} else {
				scIDs = append(scIDs, ids...)
				log.Infof("Got %d id(s) from %s", len(ids), si.Address)
			}
		}
		sort.Slice(scIDs, func(i, j int) bool {
			return bytes.Compare(scIDs[i], scIDs[j]) < 0
		})
		for i := len(scIDs) - 1; i > 0; i-- {
			if scIDs[i].Equal(scIDs[i-1]) {
				scIDs = append(scIDs[0:i], scIDs[i+1:]...)
			}
		}
		log.Info("All IDs available in this roster:")
		for _, id := range scIDs {
			log.Infof("%x", id[:])
		}
	} else {
		id, err := hex.DecodeString(c.Args().Get(1))
		if err != nil || len(id) != 32 {
			return errors.New("second argument is not a valid ID")
		}
		var cl *byzcoin.Client
		var cc *byzcoin.ChainConfig
		for _, si := range r.List {
			ids, err := fetchChains(si, byzcoinFetcher, skipchainFetcher)
			if err != nil {
				log.Warn("Got error while asking", si.Address, "for skipchains:", err)
			}
			found := false
			for _, idc := range ids {
				if idc.Equal(id) {
					found = true
					break
				}
			}
			if found {
				cl = byzcoin.NewClient(id, *onet.NewRoster([]*network.ServerIdentity{si}))
				cc, err = cl.GetChainConfig()
				if err != nil {
					cl = nil
					log.Warnf("Could not get chain config from %v: %v\n", si, err)
					continue
				}
				cl.Roster = cc.Roster
				break
			}
		}
		if cl == nil {
			return errors.New("didn't manage to find a node with a valid copy of the given skipchain-id")
		}

		newDarc := &darc.Darc{}

		dstr := c.String("darc")
		if dstr == "" {
			log.Info("no darc given, will use an empty default one")
		} else {

			// Accept both plain-darcs, as well as "darc:...." darcs
			darcID, err := lib.StringToDarcID(dstr)
			if err != nil {
				return errors.New("failed to parse darc: " + err.Error())
			}

			p, err := cl.GetProofFromLatest(darcID)
			if err != nil {
				return errors.New("couldn't get proof for darc: " + err.Error())
			}

			_, darcBuf, cid, _, err := p.Proof.KeyValue()
			if err != nil {
				return errors.New("cannot get value for darc: " + err.Error())
			}

			if cid != byzcoin.ContractDarcID {
				return errors.New("please give a darc-instance ID, not: " + cid)
			}

			newDarc, err = darc.NewFromProtobuf(darcBuf)
			if err != nil {
				return errors.New("invalid darc stored in byzcoin: " + err.Error())
			}
		}

		identity := cothority.Suite.Point()

		identityStr := c.String("identity")
		if identityStr == "" {
			log.Info("no identity provided, will use a default one")
		} else {
			identityBuf, err := lib.StringToEd25519Buf(identityStr)
			if err != nil {
				return err
			}

			identity = cothority.Suite.Point()
			err = identity.UnmarshalBinary(identityBuf)
			if err != nil {
				return errors.New("got an invalid identity: " + err.Error())
			}
		}

		log.Infof("ByzCoin-config for %+x:\n"+
			"\tRoster: %s\n"+
			"\tBlockInterval: %s\n"+
			"\tMacBlockSize: %d\n"+
			"\tDarcContracts: %s",
			id[:], cc.Roster.List, cc.BlockInterval, cc.MaxBlockSize, cc.DarcContractIDs)
		filePath, err := lib.SaveConfig(lib.Config{
			Roster:        cc.Roster,
			ByzCoinID:     id,
			AdminDarc:     *newDarc,
			AdminIdentity: darc.NewIdentityEd25519(identity),
		})
		if err != nil {
			return errors.New("while writing config-file: " + err.Error())
		}
		log.Info(fmt.Sprintf("Wrote config to \"%s\"", filePath))
	}
	return nil
}

func byzcoinFetcher(si *network.ServerIdentity) ([]skipchain.SkipBlockID, error) {
	cl := byzcoin.NewClient(nil, onet.Roster{})
	reply, err := cl.GetAllByzCoinIDs(si)
	if err != nil {
		return nil, err
	}

	return reply.IDs, nil
}

func skipchainFetcher(si *network.ServerIdentity) ([]skipchain.SkipBlockID, error) {
	cl := skipchain.NewClient()
	reply, err := cl.GetAllSkipChainIDs(si)
	if err != nil {
		return nil, err
	}

	return reply.IDs, nil
}

func fetchChains(si *network.ServerIdentity, fns ...chainFetcher) ([]skipchain.SkipBlockID, error) {
	for _, fn := range fns {
		ids, err := fn(si)
		if err != nil {
			if !strings.Contains(err.Error(), errUnregisteredMessage) {
				return nil, err
			}
		} else {
			return ids, nil
		}
	}

	return nil, errors.New("couldn't find registered handler")
}

func latest(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		bcArg = c.Args().First()
		if bcArg == "" {
			return errors.New("--bc flag is required")
		}
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	// Allow the user to set the server number; useful when testing leader rotation.
	cl.ServerNumber = c.Int("server")
	if cl.ServerNumber > len(cl.Roster.List)-1 {
		return errors.New("server index out of range")
	}

	_, err = fmt.Fprintf(c.App.Writer, "ByzCoinID: %x\n", cfg.ByzCoinID)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.App.Writer, "Admin DARC: %x\n", cfg.AdminDarc.GetBaseID())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.App.Writer, "local roster:", fmtRoster(&cfg.Roster))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.App.Writer, "contacting server:", cl.Roster.List[cl.ServerNumber])
	if err != nil {
		return err
	}

	// Find the latest block by asking for the Proof of the config instance.
	p, err := cl.GetProofFromLatest(byzcoin.ConfigInstanceID.Slice())
	if err != nil {
		return err
	}

	sb := p.Proof.Latest
	_, err = fmt.Fprintf(c.App.Writer, "Last block:\n\tIndex: %d\n\tBlockMaxHeight: %d\n\tBackLinks: %d\n\tRoster: %s\n\n",
		sb.Index, sb.Height, len(sb.BackLinkIDs), fmtRoster(sb.Roster))
	if err != nil {
		return err
	}

	if c.Bool("roster") {
		g := &app.Group{Roster: sb.Roster}
		gt, err := g.Toml(cothority.Suite)
		if err != nil {
			return err
		}
		fmt.Fprintln(c.App.Writer, gt.String())
	}

	if c.Bool("update") {
		cfg.Roster = *sb.Roster
		var fn string
		fn, err = lib.SaveConfig(cfg)
		if err == nil {
			_, err = fmt.Fprintln(c.App.Writer, "updated config file:", fn)
			if err != nil {
				return err
			}
		}
	}

	return err
}

func fmtRoster(r *onet.Roster) string {
	var roster []string
	for _, s := range r.List {
		if s.URL != "" {
			roster = append(roster, fmt.Sprintf("%v (url: %v)", string(s.Address), s.URL))
		} else {
			roster = append(roster, string(s.Address))
		}
	}
	return strings.Join(roster, ", ")
}

func getBcKey(c *cli.Context) (cfg lib.Config, cl *byzcoin.Client, signer *darc.Signer,
	proof byzcoin.Proof, chainCfg byzcoin.ChainConfig, err error) {
	if c.NArg() < 2 {
		err = errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg")
		return
	}
	cfg, cl, err = lib.LoadConfig(c.Args().First())
	if err != nil {
		err = errors.New("couldn't load config file: " + err.Error())
		return
	}
	signer, err = lib.LoadSigner(c.Args().Get(1))
	if err != nil {
		err = errors.New("couldn't load key-xxx.cfg: " + err.Error())
		return
	}

	log.Lvl2("Getting latest chainConfig")
	pr, err := cl.GetProofFromLatest(byzcoin.ConfigInstanceID.Slice())
	if err != nil {
		err = errors.New("couldn't get proof for chainConfig: " + err.Error())
		return
	}
	proof = pr.Proof

	_, value, _, _, err := proof.KeyValue()
	if err != nil {
		err = errors.New("couldn't get value out of proof: " + err.Error())
		return
	}
	err = protobuf.DecodeWithConstructors(value, &chainCfg, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		err = errors.New("couldn't decode chainConfig: " + err.Error())
		return
	}
	return
}

func getBcKeyPub(c *cli.Context) (cfg lib.Config, cl *byzcoin.Client, signer *darc.Signer,
	proof byzcoin.Proof, chainCfg byzcoin.ChainConfig, pub *network.ServerIdentity, err error) {
	cfg, cl, signer, proof, chainCfg, err = getBcKey(c)
	if err != nil {
		return
	}

	fn := c.Args().Get(2)
	if fn == "" {
		err = errors.New("no TOML file provided")
		return
	}
	f, err := os.Open(fn)
	if err != nil {
		return
	}
	defer f.Close()
	group, err := app.ReadGroupDescToml(f)
	if err != nil {
		err = fmt.Errorf("couldn't open %v: %v", fn, err.Error())
		return
	}
	if len(group.Roster.List) != 1 {
		err = errors.New("the TOML file should have exactly one entry")
		return
	}
	pub = group.Roster.List[0]

	return
}

func updateConfig(cl *byzcoin.Client, signer *darc.Signer, chainConfig byzcoin.ChainConfig) error {
	counters, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return errors.New("couldn't get counters: " + err.Error())
	}
	counters.Counters[0]++
	ccBuf, err := protobuf.Encode(&chainConfig)
	if err != nil {
		return errors.New("couldn't encode chainConfig: " + err.Error())
	}
	ctx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{{
			InstanceID: byzcoin.ConfigInstanceID,
			Invoke: &byzcoin.Invoke{
				ContractID: byzcoin.ContractConfigID,
				Command:    "update_config",
				Args:       byzcoin.Arguments{{Name: "config", Value: ccBuf}},
			},
			SignerCounter: counters.Counters,
		}},
	}

	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return errors.New("couldn't sign the clientTransaction: " + err.Error())
	}

	log.Lvl1("Sending new roster to byzcoin")
	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return errors.New("client transaction wasn't accepted: " + err.Error())
	}
	return nil
}

func config(c *cli.Context) error {
	_, cl, signer, _, chainConfig, err := getBcKey(c)
	if err != nil {
		return err
	}

	if interval := c.String("interval"); interval != "" {
		dur, err := time.ParseDuration(interval)
		if err != nil {
			return errors.New("couldn't parse interval: " + err.Error())
		}
		chainConfig.BlockInterval = dur
	}
	if blockSize := c.Int("blockSize"); blockSize > 0 {
		if blockSize < 16000 && blockSize > 8e6 {
			return errors.New("new blocksize out of bounds: must be between 16e3 and 8e6")
		}
		chainConfig.MaxBlockSize = blockSize
	}

	err = updateConfig(cl, signer, chainConfig)
	if err != nil {
		return err
	}

	log.Lvl1("Updated configuration")

	return nil
}

func mint(c *cli.Context) error {
	if c.NArg() < 4 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg pubkey coins")
	}
	cfg, cl, signer, _, _, err := getBcKey(c)
	if err != nil {
		return err
	}

	pubBuf, err := hex.DecodeString(c.Args().Get(2))
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write([]byte(contracts.ContractCoinID))
	h.Write(pubBuf)
	account := byzcoin.NewInstanceID(h.Sum(nil))

	coins, err := strconv.ParseUint(c.Args().Get(3), 10, 64)
	if err != nil {
		return err
	}
	coinsBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(coinsBuf, coins)

	cReply, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return err
	}
	counters := cReply.Counters

	p, err := cl.GetProofFromLatest(account.Slice())
	if err != nil {
		return err
	}
	if !p.Proof.InclusionProof.Match(account.Slice()) {
		log.Info("Creating darc and coin")
		pub := cothority.Suite.Point()
		err = pub.UnmarshalBinary(pubBuf)
		if err != nil {
			return err
		}
		pubI := darc.NewIdentityEd25519(pub)
		rules := darc.NewRules()
		err = rules.AddRule(darc.Action("spawn:coin"), expression.Expr(signer.Identity().String()))
		if err != nil {
			return err
		}
		err = rules.AddRule(darc.Action("invoke:coin.transfer"), expression.Expr(pubI.String()))
		if err != nil {
			return err
		}
		err = rules.AddRule(darc.Action("invoke:coin.mint"), expression.Expr(signer.Identity().String()))
		if err != nil {
			return err
		}
		d := darc.NewDarc(rules, []byte("new coin for mba"))
		dBuf, err := d.ToProto()
		if err != nil {
			return err
		}

		log.Info("Creating darc for coin")
		counters[0]++
		ctx := byzcoin.ClientTransaction{
			Instructions: byzcoin.Instructions{{
				InstanceID: byzcoin.NewInstanceID(cfg.AdminDarc.GetBaseID()),
				Spawn: &byzcoin.Spawn{
					ContractID: byzcoin.ContractDarcID,
					Args: byzcoin.Arguments{{
						Name:  "darc",
						Value: dBuf,
					}},
				},
				SignerCounter: counters,
			}},
		}
		err = ctx.FillSignersAndSignWith(*signer)
		if err != nil {
			return err
		}
		_, err = cl.AddTransactionAndWait(ctx, 10)
		if err != nil {
			return err
		}

		log.Info("Creating coin")
		counters[0]++
		ctx = byzcoin.ClientTransaction{
			Instructions: byzcoin.Instructions{{
				InstanceID: byzcoin.NewInstanceID(d.GetBaseID()),
				Spawn: &byzcoin.Spawn{
					ContractID: contracts.ContractCoinID,
					Args: byzcoin.Arguments{
						{
							Name:  "type",
							Value: contracts.CoinName.Slice(),
						},
						{
							Name:  "coinID",
							Value: pubBuf,
						},
					},
				},
				SignerCounter: counters,
			}},
		}
		err = ctx.FillSignersAndSignWith(*signer)
		if err != nil {
			return err
		}
		_, err = cl.AddTransactionAndWait(ctx, 10)
		if err != nil {
			return err
		}
	}

	log.Info("Minting coin")
	counters[0]++
	ctx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{{
			InstanceID: account,
			Invoke: &byzcoin.Invoke{
				ContractID: contracts.ContractCoinID,
				Command:    "mint",
				Args: byzcoin.Arguments{{
					Name:  "coins",
					Value: coinsBuf,
				}},
			},
			SignerCounter: counters,
		}},
	}
	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}
	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	log.Infof("Account %x created and filled with %d coins", account[:], coins)
	return nil
}

func rosterAdd(c *cli.Context) error {
	if c.NArg() < 3 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg newServer.toml")
	}
	_, cl, signer, _, chainConfig, pub, err := getBcKeyPub(c)
	if err != nil {
		return err
	}

	old := chainConfig.Roster
	if i, _ := old.Search(pub.ID); i >= 0 {
		return errors.New("new node is already in roster")
	}
	log.Lvl2("Old roster is:", old.List)
	chainConfig.Roster = *old.Concat(pub)
	log.Lvl2("New roster is:", chainConfig.Roster.List)

	err = updateConfig(cl, signer, chainConfig)
	if err != nil {
		return err
	}
	log.Lvl1("New roster is now active")
	return nil
}

func rosterDel(c *cli.Context) error {
	if c.NArg() < 3 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg serverToDelete.toml")
	}
	_, cl, signer, _, chainConfig, pub, err := getBcKeyPub(c)
	if err != nil {
		return err
	}

	old := chainConfig.Roster
	i, _ := old.Search(pub.ID)
	switch {
	case i < 0:
		return errors.New("node to delete is not in roster")
	case i == 0:
		return errors.New("cannot delete leader from roster")
	}
	log.Lvl2("Old roster is:", old.List)
	list := append(old.List[0:i], old.List[i+1:]...)
	chainConfig.Roster = *onet.NewRoster(list)
	log.Lvl2("New roster is:", chainConfig.Roster.List)

	err = updateConfig(cl, signer, chainConfig)
	if err != nil {
		return err
	}
	log.Lvl1("New roster is now active")
	return nil
}

func rosterLeader(c *cli.Context) error {
	if c.NArg() < 3 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg newLeader.toml")
	}
	_, cl, signer, _, chainConfig, pub, err := getBcKeyPub(c)
	if err != nil {
		return err
	}

	old := chainConfig.Roster
	i, _ := old.Search(pub.ID)
	switch {
	case i < 0:
		return errors.New("new leader is not in roster")
	case i == 0:
		return errors.New("new node is already leader")
	}
	log.Lvl2("Old roster is:", old.List)
	list := []*network.ServerIdentity(old.List)
	list[0], list[i] = list[i], list[0]
	chainConfig.Roster = *onet.NewRoster(list)
	log.Lvl2("New roster is:", chainConfig.Roster.List)

	// Do it twice to make sure the new roster is active - there is an issue ;)
	err = updateConfig(cl, signer, chainConfig)
	if err != nil {
		return err
	}
	err = updateConfig(cl, signer, chainConfig)
	if err != nil {
		return err
	}
	log.Lvl1("New roster is now active")
	return nil
}

func key(c *cli.Context) error {
	if f := c.String("print"); f != "" {
		sig, err := lib.LoadSigner(f)
		if err != nil {
			return errors.New("couldn't load signer: " + err.Error())
		}
		log.Infof("Private: %s\nPublic: %s", sig.Ed25519.Secret, sig.Ed25519.Point)
		return nil
	}
	newSigner := darc.NewSignerEd25519(nil, nil)
	err := lib.SaveKey(newSigner)
	if err != nil {
		return err
	}

	var fo io.Writer

	save := c.String("save")
	if save == "" {
		fo = os.Stdout
	} else {
		file, err := os.Create(save)
		if err != nil {
			return err
		}
		fo = file
		defer func() {
			err := file.Close()
			if err != nil {
				log.Error(err)
			}
		}()
	}
	_, err = fmt.Fprintln(fo, newSigner.Identity().String())
	return err
}

func darcShow(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	dstr := c.String("darc")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}

	d, err := lib.GetDarcByString(cl, dstr)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.App.Writer, d.String())
	return err
}

func debugReplay(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("please give the following arguments: url [bcID]")
	}
	if c.NArg() == 1 {
		err := debugList(c)
		if err != nil {
			return err
		}

		log.Info("Please provide one of the following byzcoin ID as the second argument")
		return nil
	}

	r := &onet.Roster{List: []*network.ServerIdentity{{
		URL: c.Args().First(),
		// valid server identity must have a public so we create a fake one
		// as we are only interested in the URL.
		Public: cothority.Suite.Point().Base(),
	}}}
	if r == nil {
		return errors.New("couldn't create roster")
	}
	bcID, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return err
	}

	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	servers := local.GenServers(1)
	s := servers[0].Service(byzcoin.ServiceName).(*byzcoin.Service)

	cl := skipchain.NewClient()
	stack := []*skipchain.SkipBlock{}
	cb := func(ro *onet.Roster, sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
		if len(stack) > 0 {
			// Use the blocks stored locally if possible ..
			sb := stack[0]
			stack = stack[1:]

			// .. but only if it matches.
			if sb.Hash.Equal(sib) {
				return sb, nil
			}
		}

		// Try to get more than a block at once to speed up the process.
		blocks, err := cl.GetUpdateChainLevel(ro, sib, 1, 50)
		if err != nil {
			log.Info("An error occurred when getting the chain. Trying a single block.")
			// In the worst case, it fetches only the requested block.
			return cl.GetSingleBlock(ro, sib)
		}

		stack = blocks[1:]
		return blocks[0], nil
	}

	log.Info("Replaying blocks")
	_, err = s.ReplayState(bcID, r, cb)
	if err != nil {
		return err
	}
	log.Info("Successfully checked and replayed all blocks.")

	return err
}

// "cDesc" stands for Change Description. This function allows one to edit the
// description of a darc.
func darcCdesc(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	desc := c.String("desc")
	if desc == "" {
		return errors.New("--desc flag is required")
	}
	if len(desc) > 1024 {
		return errors.New("descriptions longer than 1024 characters are not allowed")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	dstr := c.String("dstr")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}

	d, err := lib.GetDarcByString(cl, dstr)
	if err != nil {
		return err
	}

	d2 := d.Copy()
	err = d2.EvolveFrom(d)
	if err != nil {
		return err
	}

	d2.Description = []byte(desc)
	d2Buf, err := d2.ToProto()
	if err != nil {
		return err
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())

	invoke := byzcoin.Invoke{
		ContractID: byzcoin.ContractDarcID,
		Command:    "evolve",
		Args: []byzcoin.Argument{
			{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{
			{
				InstanceID:    byzcoin.NewInstanceID(d2.GetBaseID()),
				Invoke:        &invoke,
				SignerCounter: []uint64{counters.Counters[0] + 1},
			},
		},
	}
	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	return nil
}

func debugList(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("please give (ip:port | group.toml) as argument")
	}

	var urls []string
	if f, err := os.Open(c.Args().First()); err == nil {
		defer f.Close()
		group, err := app.ReadGroupDescToml(f)
		if err != nil {
			return err
		}
		for _, si := range group.Roster.List {
			if si.URL != "" {
				urls = append(urls, si.URL)
			} else {
				p, err := strconv.Atoi(si.Address.Port())
				if err != nil {
					return err
				}
				urls = append(urls, fmt.Sprintf("http://%s:%d", si.Address.Host(), p+1))
			}
		}
	} else {
		urls = []string{c.Args().First()}
	}

	for _, url := range urls {
		log.Info("Contacting ", url)
		resp, err := byzcoin.Debug(url, nil)
		if err != nil {
			log.Error(err)
			continue
		}
		sort.SliceStable(resp.Byzcoins, func(i, j int) bool {
			var iData byzcoin.DataHeader
			var jData byzcoin.DataHeader
			err := protobuf.Decode(resp.Byzcoins[i].Genesis.Data, &iData)
			if err != nil {
				return false
			}
			err = protobuf.Decode(resp.Byzcoins[j].Genesis.Data, &jData)
			if err != nil {
				return false
			}
			return iData.Timestamp > jData.Timestamp
		})
		for _, rb := range resp.Byzcoins {
			log.Infof("ByzCoinID %x has", rb.ByzCoinID)
			headerGenesis := byzcoin.DataHeader{}
			headerLatest := byzcoin.DataHeader{}
			err := protobuf.Decode(rb.Genesis.Data, &headerGenesis)
			if err != nil {
				log.Error(err)
				continue
			}
			err = protobuf.Decode(rb.Latest.Data, &headerLatest)
			if err != nil {
				log.Error(err)
				continue
			}
			log.Infof("\tBlocks: %d\n\tFrom %s to %s\tBlock hash: %x",
				rb.Latest.Index,
				time.Unix(headerGenesis.Timestamp/1e9, 0),
				time.Unix(headerLatest.Timestamp/1e9, 0),
				rb.Latest.Hash[:])
			if c.Bool("verbose") {
				log.Infof("\tRoster: %s\n\tGenesis block header: %+v\n\tLatest block header: %+v",
					rb.Latest.Roster.List,
					rb.Genesis.SkipBlockFix,
					rb.Latest.SkipBlockFix)
			}
			log.Info()
		}
	}
	return nil
}

func debugDump(c *cli.Context) error {
	if c.NArg() < 2 {
		return errors.New("please give the following arguments: ip:port byzcoin-id")
	}

	bcidBuf, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		log.Error(err)
		return err
	}
	bcid := skipchain.SkipBlockID(bcidBuf)
	resp, err := byzcoin.Debug(c.Args().First(), &bcid)
	if err != nil {
		log.Error(err)
		return err
	}
	sort.SliceStable(resp.Dump, func(i, j int) bool {
		return bytes.Compare(resp.Dump[i].Key, resp.Dump[j].Key) < 0
	})
	for _, inst := range resp.Dump {
		log.Infof("%x / %d: %s", inst.Key, inst.State.Version, string(inst.State.ContractID))
		if c.Bool("verbose") {
			switch inst.State.ContractID {
			case byzcoin.ContractDarcID:
				d, err := darc.NewFromProtobuf(inst.State.Value)
				if err != nil {
					log.Warn("Didn't recognize as a darc instance")
				}
				log.Infof("\tDesc: %s, Rules:", string(d.Description))
				for _, r := range d.Rules.List {
					log.Infof("\tAction: %s - Expression: %s", r.Action, r.Expr)
				}
			}
		}
	}

	return nil
}

func debugRemove(c *cli.Context) error {
	if c.NArg() < 2 {
		return errors.New("please give the following arguments: private.toml byzcoin-id")
	}

	ccfg, err := app.LoadCothority(c.Args().First())
	if err != nil {
		return err
	}
	si, err := ccfg.GetServerIdentity()
	if err != nil {
		return err
	}
	bcidBuf, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		log.Error(err)
		return err
	}
	bcid := skipchain.SkipBlockID(bcidBuf)
	err = byzcoin.DebugRemove(si, bcid)
	if err != nil {
		return err
	}
	log.Infof("Successfully removed ByzCoinID %x from %s", bcid, si.Address)
	return nil
}

func darcAdd(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	dstr := c.String("darc")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}
	dSpawn, err := lib.GetDarcByString(cl, dstr)
	if err != nil {
		return err
	}

	var signer *darc.Signer

	identities := c.StringSlice("identity")

	if len(identities) == 0 {
		s := darc.NewSignerEd25519(nil, nil)
		err = lib.SaveKey(s)
		if err != nil {
			return err
		}
		identities = append(identities, s.Identity().String())
	}

	Y := expression.InitParser(func(s string) bool { return true })

	for _, id := range identities {
		expr := []byte(id)
		_, err := expression.Evaluate(Y, expr)
		if err != nil {
			return errors.New("failed to parse id: " + err.Error())
		}
	}

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	var desc []byte
	if c.String("desc") == "" {
		desc = []byte(randString(10))
	} else {
		if len(c.String("desc")) > 1024 {
			return errors.New("descriptions longer than 1024 characters are not allowed")
		}
		desc = []byte(c.String("desc"))
	}

	deferredExpr := expression.InitOrExpr(identities...)

	adminExpr := expression.InitAndExpr(identities...)

	rules := darc.NewRules()
	rules.AddRule("invoke:"+byzcoin.ContractDarcID+".evolve", adminExpr)
	rules.AddRule("_sign", adminExpr)
	if c.Bool("deferred") {
		rules.AddRule("spawn:deferred", deferredExpr)
		rules.AddRule("invoke:deferred.addProof", deferredExpr)
		rules.AddRule("invoke:deferred.execProposedTx", deferredExpr)
	}
	if c.Bool("unrestricted") {
		err = rules.AddRule("invoke:"+byzcoin.ContractDarcID+".evolve_unrestricted", adminExpr)
		if err != nil {
			return err
		}
	}

	d := darc.NewDarc(rules, desc)

	dBuf, err := d.ToProto()
	if err != nil {
		return err
	}

	instID := byzcoin.NewInstanceID(dSpawn.GetBaseID())

	counters, err := cl.GetSignerCounters(signer.Identity().String())

	spawn := byzcoin.Spawn{
		ContractID: byzcoin.ContractDarcID,
		Args: []byzcoin.Argument{
			{
				Name:  "darc",
				Value: dBuf,
			},
		},
	}

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{
			{
				InstanceID:    instID,
				Spawn:         &spawn,
				SignerCounter: []uint64{counters.Counters[0] + 1},
			},
		},
	}
	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.App.Writer, d.String())
	if err != nil {
		return err
	}

	// Saving ID in special file
	output := c.String("out_id")
	if output != "" {
		err = ioutil.WriteFile(output, []byte(d.GetIdentityString()), 0644)
		if err != nil {
			return err
		}
	}

	// Saving key in special file
	output = c.String("out_key")
	if len(c.StringSlice("identity")) == 0 && output != "" {
		err = ioutil.WriteFile(output, []byte(identities[0]), 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

func darcRule(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	dstr := c.String("darc")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}
	d, err := lib.GetDarcByString(cl, dstr)
	if err != nil {
		return err
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	action := c.String("rule")
	if action == "" {
		return errors.New("--rule flag is required")
	}

	identities := c.StringSlice("identity")

	if len(identities) == 0 {
		if !c.Bool("delete") {
			return errors.New("--identity flag is required")
		}
	}

	Y := expression.InitParser(func(s string) bool { return true })

	for _, id := range identities {
		expr := []byte(id)
		_, err := expression.Evaluate(Y, expr)
		if err != nil {
			return errors.New("failed to parse id: " + err.Error())
		}
	}

	var groupExpr expression.Expr
	min := c.Uint("minimum")
	if min == 0 {
		groupExpr = expression.InitAndExpr(identities...)
	} else {
		andGroups := lib.CombinationAnds(identities, int(min))
		groupExpr = expression.InitOrExpr(andGroups...)
	}

	d2 := d.Copy()
	err = d2.EvolveFrom(d)
	if err != nil {
		return err
	}

	switch {
	case c.Bool("delete"):
		err = d2.Rules.DeleteRules(darc.Action(action))
	case c.Bool("replace"):
		err = d2.Rules.UpdateRule(darc.Action(action), groupExpr)
	default:
		err = d2.Rules.AddRule(darc.Action(action), groupExpr)
	}

	if err != nil {
		return err
	}

	d2Buf, err := d2.ToProto()
	if err != nil {
		return err
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())

	invoke := byzcoin.Invoke{
		ContractID: byzcoin.ContractDarcID,
		Command:    "evolve_unrestricted",
		Args: []byzcoin.Argument{
			{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{
			{
				InstanceID:    byzcoin.NewInstanceID(d2.GetBaseID()),
				Invoke:        &invoke,
				SignerCounter: []uint64{counters.Counters[0] + 1},
			},
		},
	}
	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	return nil
}

// print a rule based on the identities and the minimum given.
func darcPrintRule(c *cli.Context) error {

	identities := c.StringSlice("identity")

	if len(identities) == 0 {
		if !c.Bool("delete") {
			return errors.New("--identity (-id) flag is required")
		}
	}

	Y := expression.InitParser(func(s string) bool { return true })

	for _, id := range identities {
		expr := []byte(id)
		_, err := expression.Evaluate(Y, expr)
		if err != nil {
			return errors.New("failed to parse id: " + err.Error())
		}
	}

	var groupExpr expression.Expr
	min := c.Uint("minimum")
	if min == 0 {
		groupExpr = expression.InitAndExpr(identities...)
	} else {
		andGroups := lib.CombinationAnds(identities, int(min))
		groupExpr = expression.InitOrExpr(andGroups...)
	}

	fmt.Fprintf(c.App.Writer, "%s\n", groupExpr)

	return nil
}

func qrcode(c *cli.Context) error {
	type pair struct {
		Priv string
		Pub  string
	}
	type baseconfig struct {
		ByzCoinID skipchain.SkipBlockID
	}

	type adminconfig struct {
		ByzCoinID skipchain.SkipBlockID
		Admin     pair
	}

	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, _, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	var toWrite []byte

	if c.Bool("admin") {
		signer, err := lib.LoadKey(cfg.AdminIdentity)
		if err != nil {
			return err
		}

		priv, err := signer.GetPrivate()
		if err != nil {
			return err
		}

		toWrite, err = json.Marshal(adminconfig{
			ByzCoinID: cfg.ByzCoinID,
			Admin: pair{
				Priv: priv.String(),
				Pub:  signer.Identity().String(),
			},
		})
	} else {
		toWrite, err = json.Marshal(baseconfig{
			ByzCoinID: cfg.ByzCoinID,
		})
	}

	if err != nil {
		return err
	}

	qr, err := qrgo.NewQR(string(toWrite))
	if err != nil {
		return err
	}

	qr.OutputTerminal()

	return nil
}

func getInfo(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, _, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	log.Infof("BC configuration:\n"+
		"\tCongig path: %s\n"+
		"\tRoster: %s\n"+
		"\tByzCoinID: %x\n"+
		"\tDarc Base ID: %x\n"+
		"\tIdentity: %s\n",
		bcArg, cfg.Roster.List, cfg.ByzCoinID, cfg.AdminDarc.GetBaseID(), cfg.AdminIdentity.String())

	return nil
}

type configPrivate struct {
	Owner darc.Signer
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bigN := big.NewInt(int64(len(letters)))
	b := make([]byte, n)
	r := random.New()
	for i := range b {
		x := int(random.Int(bigN, r).Int64())
		b[i] = letters[x]
	}
	return string(b)
}

func init() { network.RegisterMessages(&configPrivate{}) }
