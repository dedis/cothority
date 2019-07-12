package main

import (
	"go.dedis.ch/cothority/v3/calypso/csadmin/clicontracts"
	"gopkg.in/urfave/cli.v1"
)

var cmds = cli.Commands{
	{
		Name:      "authorize",
		Usage:     "store the byzcoin-id that should be trusted to create new LTS",
		Aliases:   []string{"a"},
		ArgsUsage: "private.toml",
		Action:    authorize,
	},
	{
		Name:  "dkg",
		Usage: "handles DKG operations",
		Subcommands: cli.Commands{
			{
				Name:   "start",
				Usage:  "starts a DKG given the instance ID of an LTS",
				Action: dkgStart,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:   "bc",
						EnvVar: "BC",
						Usage:  "the ByzCoin config to use (required)",
					},
					cli.StringFlag{
						Name:  "instid, i",
						Usage: "the instance id of the spawned LTS contract",
					},
					cli.BoolFlag{
						Name:  "export, x",
						Usage: "exports the public key to STDOUT",
					},
				},
			},
		},
	},
	{
		Name:   "reencrypt",
		Usage:  "decrypt and reencrypt the secret of a write instance given the proofs of write and read instances",
		Action: reencrypt,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use (required)",
			},
			cli.StringFlag{
				Name:  "writeid, w",
				Usage: "instance id of the write instance",
			},
			cli.StringFlag{
				Name:  "readid, r",
				Usage: "instance id of the read instance",
			},
			cli.BoolFlag{
				Name:  "export, x",
				Usage: "exports the DecryptReply to STDOUT",
			},
		},
	},
	{
		Name:   "decrypt",
		Usage:  "decrypt a re-encrypted key given a DecryptKeyReply struct read from STDIN ",
		Action: decrypt,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use (required)",
			},
			cli.StringFlag{
				Name:  "key",
				Usage: "path to the private.toml file (default is admin key)",
			},
			cli.BoolFlag{
				Name:  "export, x",
				Usage: "exports the decrypted data to STDOUT",
			},
		},
	},
	{
		Name:  "contract",
		Usage: "Provides cli interface for contracts",
		Subcommands: cli.Commands{
			{
				Name:  "lts",
				Usage: "handle LTS contract",
				Subcommands: cli.Commands{
					{
						Name:   "spawn",
						Usage:  "spawn an LTS contract",
						Action: clicontracts.LTSSpawn,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
							cli.StringFlag{
								Name:  "darc",
								Usage: "DARC with the right to create an LTS (default is the admin DARC)",
							},
							cli.StringFlag{
								Name:  "sign, s",
								Usage: "public key of the signing entity (default is the admin)",
							},
							cli.BoolFlag{
								Name:  "export, x",
								Usage: "export the instance id to STDOUT",
							},
						},
					},
				},
			},
			{
				Name:  "write",
				Usage: "handles write contract",
				Subcommands: cli.Commands{
					{
						Name:   "spawn",
						Usage:  "spawn a write contract. Reads the public key from STDIN.",
						Action: clicontracts.WriteSpawn,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
							cli.StringFlag{
								Name:  "darc",
								Usage: "DARC with the right to create a Write instance (default is the admin DARC)",
							},
							cli.StringFlag{
								Name:  "sign, s",
								Usage: "public key of the signing entity (default is the admin)",
							},
							cli.StringFlag{
								Name:  "instid, i",
								Usage: "the instance id of the spawned LTS contract",
							},
							cli.StringFlag{
								Name:  "secret",
								Usage: "data to be encrypted, has limited space regarding the kyber.Suite used (29 bits for ed25519)",
							},
							cli.StringFlag{
								Name:  "data",
								Usage: "cleartext data that can be stored in the Write contract (optional). Is not used if --readin is provided.",
							},
							cli.BoolFlag{
								Name:  "readin",
								Usage: "if provided, the --data flag is not used and the data is read from STDIN",
							},
							cli.StringFlag{
								Name:  "key",
								Usage: "hexadecimal LTS public key",
							},
							cli.BoolFlag{
								Name:  "export, x",
								Usage: "export the instance id to STDOUT",
							},
						},
					},
					{
						Name:   "get",
						Usage:  "if the proof matches, prints the content of the given Write instance ID",
						Action: clicontracts.WriteGet,
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
				},
			},
			{
				Name:  "read",
				Usage: "handles read contract",
				Subcommands: cli.Commands{
					{
						Name:   "spawn",
						Usage:  "do not really spawn a read contract, but calls the spawn of the write contract",
						Action: clicontracts.ReadSpawn,
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:   "bc",
								EnvVar: "BC",
								Usage:  "the ByzCoin config to use (required)",
							},
							cli.StringFlag{
								Name:  "sign, s",
								Usage: "public key of the signing entity (default is the admin)",
							},
							cli.StringFlag{
								Name:  "instid, i",
								Usage: "the instance id of the Write contract",
							},
							cli.StringFlag{
								Name:  "key",
								Usage: "hexadecimal public key (if not provided, use the signer's key)",
							},
							cli.BoolFlag{
								Name:  "export, x",
								Usage: "export the instance id to STDOUT",
							},
						},
					},
				},
			},
		},
	},
}
