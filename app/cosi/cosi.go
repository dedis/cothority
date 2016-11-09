// CoSi is a scalable protocol for collectively signing messages.
// CoSi produces compact signatures that clients can verify efficiently,
// and that convey the precise set of cosigners for transparency.
package main

import (
	"os"
	"time"

	"github.com/dedis/cothority/app/lib/server"
	"github.com/dedis/cothority/log"
	"gopkg.in/urfave/cli.v1"
)

const (
	// BinaryName represents the Name of the binary
	BinaryName = "cosi"

	// Version of the binary
	Version = "0.10"

	// DefaultGroupFile is the name of the default file to lookup for group
	// definition
	DefaultGroupFile = "group.toml"

	optionGroup      = "group"
	optionGroupShort = "g"

	optionConfig      = "config"
	optionConfigShort = "c"

	// RequestTimeOut defines when the client stops waiting for the CoSi group to
	// reply
	RequestTimeOut = time.Second * 10
)

func main() {
	app := cli.NewApp()
	app.Name = "CoSi app"
	app.Usage = "Collectively sign a file or verify its signature."
	app.Version = Version
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
			Value: DefaultGroupFile,
			Usage: "CoSi group definition file",
		},
	}

	serverFlags := []cli.Flag{
		cli.StringFlag{
			Name:  optionConfig + ", " + optionConfigShort,
			Value: server.GetDefaultConfigFile(BinaryName),
			Usage: "Configuration file of the server",
		},
	}
	app.Commands = []cli.Command{
		// BEGIN CLIENT ----------
		{
			Name:    "sign",
			Aliases: []string{"s"},
			Usage:   "Collectively sign a 'msgFile'. The signature is written to STDOUT by default.",
			Action:  signFile,
			Flags: append(clientFlags, []cli.Flag{
				cli.StringFlag{
					Name:  "out, o",
					Usage: "Write signature to 'sig' instead of STDOUT.",
				},
			}...),
		},
		{
			Name:      "verify",
			Aliases:   []string{"v"},
			Usage:     "Verify collective signature of a 'msgFile'. Signature is read by default from STDIN.",
			ArgsUsage: "msgFile",
			Action:    verifyFile,
			Flags: append(clientFlags, []cli.Flag{
				cli.StringFlag{
					Name:  "signature, s",
					Usage: "Read signature from 'sig' instead of STDIN",
				},
			}...),
		},
		{
			Name:    "check",
			Aliases: []string{"c"},
			Usage:   "Check if the servers in the group definition are up and running",
			Action:  checkConfig,
			Flags:   clientFlags,
		},

		// CLIENT END ----------
		// BEGIN SERVER --------
		{
			Name:  "server",
			Usage: "act as Cothority server",
			Action: func(c *cli.Context) error {
				runServer(c)
				return nil
			},
			Flags: serverFlags,
			Subcommands: []cli.Command{
				{
					Name:    "setup",
					Aliases: []string{"s"},
					Usage:   "Setup the configuration for the server (interactive)",
					Action: func(c *cli.Context) error {
						if c.String(optionConfig) != "" {
							log.Fatal("[-] Configuration file option can't be used for the 'setup' command")
						}
						if c.GlobalIsSet("debug") {
							log.Fatal("[-] Debug option can't be used for the 'setup' command")
						}
						server.InteractiveConfig(BinaryName)
						return nil
					},
				},
			},
		},
		// SERVER END ----------
	}

	app.Flags = binaryFlags
	app.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.GlobalInt("debug"))
		return nil
	}
	err := app.Run(os.Args)
	log.ErrFatal(err)
}
