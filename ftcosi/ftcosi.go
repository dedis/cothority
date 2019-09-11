// ftCoSi is a fault-tolerant and scalable protocol for collectively signing
// messages.  It produces compact signatures that clients can verify
// efficiently, and that convey the precise set of cosigners for transparency.
package main

import (
	"os"
	"path"
	"time"

	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
)

const (
	// BinaryName represents the Name of the binary
	BinaryName = "ftcosi"

	// Version of the binary
	Version = "1.00"

	optionGroup      = "group"
	optionGroupShort = "g"

	optionConfig      = "config"
	optionConfigShort = "c"

	// RequestTimeOut defines when the client stops waiting for the CoSi group to
	// reply
	RequestTimeOut = time.Second * 10
)

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "ftcosi"
	cliApp.Usage = "collectively sign or verify a file; run a server for collective signing"
	cliApp.Version = Version
	binaryFlags := []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}

	clientFlags := []cli.Flag{
		cli.StringFlag{
			Name:  optionGroup + ", " + optionGroupShort,
			Value: app.DefaultGroupFile,
			Usage: "Cosi group definition file",
		},
	}

	serverFlags := []cli.Flag{
		cli.StringFlag{
			Name:  optionConfig + ", " + optionConfigShort,
			Value: path.Join(cfgpath.GetConfigPath(BinaryName), app.DefaultServerConfig),
			Usage: "Configuration file of the server",
		},
	}
	cliApp.Commands = []cli.Command{
		// BEGIN CLIENT ----------
		{
			Name:      "sign",
			Aliases:   []string{"s"},
			Usage:     "Request a collectively signature for a 'file'; signature is written to STDOUT by default",
			ArgsUsage: "file",
			Action:    signFile,
			Flags: append(clientFlags, []cli.Flag{
				cli.StringFlag{
					Name:  "out, o",
					Usage: "Write signature to 'file.sig' instead of STDOUT",
				},
			}...),
		},
		{
			Name:      "verify",
			Aliases:   []string{"v"},
			Usage:     "Verify a collective signature of a 'file'; signature is read from STDIN by default",
			ArgsUsage: "file",
			Action:    verifyFile,
			Flags: append(clientFlags, []cli.Flag{
				cli.StringFlag{
					Name:  "signature, s",
					Usage: "Read signature from 'file.sig' instead of STDIN",
				},
			}...),
		},
		{
			Name:    "check",
			Aliases: []string{"c"},
			Usage:   "Check if the servers in the group definition are up and running",
			Action:  checkConfig,
			Flags: append(clientFlags,
				cli.BoolFlag{
					Name:  "detail, l",
					Usage: "Show details of all servers",
				}),
		},

		// CLIENT END ----------
		// BEGIN SERVER --------
		{
			Name:  "server",
			Usage: "Start ftcosi server",
			Action: func(c *cli.Context) error {
				runServer(c)
				return nil
			},
			Flags: serverFlags,
			Subcommands: []cli.Command{
				{
					Name:    "setup",
					Aliases: []string{"s"},
					Usage:   "Setup server configuration (interactive)",
					Action: func(c *cli.Context) error {
						if c.String(optionConfig) != "" {
							log.Fatal("[-] Configuration file option cannot be used for the 'setup' command")
						}
						if c.GlobalIsSet("debug") {
							log.Fatal("[-] Debug option cannot be used for the 'setup' command")
						}
						app.InteractiveConfig(cothority.Suite, BinaryName)
						return nil
					},
				},
			},
		},
		// SERVER END ----------
	}

	cliApp.Flags = binaryFlags
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.GlobalInt("debug"))
		return nil
	}
	err := cliApp.Run(os.Args)
	log.ErrFatal(err)
}
