// Cosi takes a file or a message and signs it collectively.
// For usage, see README.md
package main

import (
	"gopkg.in/codegangsta/cli.v1"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"os"
)

// RequestTimeOut defines when the client stops waiting for the CoSi group to
// reply
const RequestTimeOut = time.Second * 10

const cothorityDef = "group"
const version = "0.1"

func init() {
	dbg.SetDebugVisible(1)
	dbg.SetUseColors(false)
}

func main() {
	app := cli.NewApp()
	app.Name = "Cosi signer and verifier"
	app.Usage = "Collectively sign a file or verify its signature."
	app.Version = version
	clientFlags := []cli.Flag{
		cli.StringFlag{
			Name:  cothorityDef + ", g",
			Value: "servers.toml",
			Usage: "Cothority group definition in `FILE.toml`: a list of servers which participate in the collective signing process",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: `integer`: 1 for terse, 5 for maximal",
		},
	}
	serverFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: getDefaultConfigFile(),
			Usage: "Configuration file of the server",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 1,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}

	app.Commands = []cli.Command{
		// BEGIN CLIENT ----------
		{
			Name:    "sign",
			Aliases: []string{"s"},
			Usage: `Collectively sign file and write signature to standard output.
	If you want to store the the signature in a file instead you can use the -out option explained below.`,
			Action: signFile,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "out, o",
					Usage: "Write signature to `outfile` instead of standard output",
				},
			},
		},
		{
			Name:    "verify",
			Aliases: []string{"v"},
			Usage:   "verify collective signature of a file",
			Action:  verifyFile,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "file, f",
					Usage: "verify signature of `FILE`",
				},
				cli.StringFlag{
					Name:  "signature, s",
					Usage: "use the `SIGNATURE_FILE` containing the signature (instead of reading from standard input)",
				},
			},
		},
		{
			Name:    "check",
			Aliases: []string{"c"},
			Usage:   "check if the servers int the group configuration are up and running",
			Action:  checkConfig,
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
						if c.String("config") != "" {
							stderrExit("[-] Configuration file option can't be used for the 'setup' command")
						}
						if c.String("debug") != "" {
							stderrExit("[-] Debug option can't be used for the 'setup' command")
						}
						interactiveConfig()
						return nil
					},
				},
			},
		},
		// SERVER END ----------

	}

	app.Flags = clientFlags
	app.Before = func(c *cli.Context) error {
		dbg.SetDebugVisible(c.GlobalInt("debug"))
		return nil
	}
	app.Run(os.Args)
}
