// Cothorityd is the main binary for running a Cothority server.
// A Cothority server can participate in various distributed protocols using the
// *cothority/sda* library with the underlying *dedis/crypto* library.
// Basically, you first need to setup a config file for the server by using:
//
// 		./cothorityd setup
//
// Then you can launch the daemon with:
//
//  	./cothorityd
//
package main

import (
	"os"

	"github.com/dedis/cothority/app/lib/server"
	"github.com/dedis/cothority/log"
	"gopkg.in/codegangsta/cli.v1"

	// Empty imports to have the init-functions called which should
	// register the protocol

	_ "github.com/dedis/cothority/protocols"
	_ "github.com/dedis/cothority/services"
)

const (
	// DefaultName is the name of the binary we produce and is used to create a directory
	// folder with this name
	DefaultName = "cothorityd"

	// Version of this binary
	Version = "1.1"
)

func main() {

	cliApp := cli.NewApp()
	cliApp.Name = "Cothorityd server"
	cliApp.Usage = "Serve a cothority"
	cliApp.Version = Version
	serverFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: server.GetDefaultConfigFile(DefaultName),
			Usage: "Configuration file of the server",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}

	cliApp.Commands = []cli.Command{
		{
			Name:    "setup",
			Aliases: []string{"s"},
			Usage:   "Setup the configuration for the server (interactive)",
			Action: func(c *cli.Context) error {
				if c.String("config") != "" {
					log.Fatal("Configuration file option can't be used for the 'setup' command")
				}
				if c.String("debug") != "" {
					log.Fatal("[-] Debug option can't be used for the 'setup' command")
				}
				server.InteractiveConfig("cothorityd")
				return nil
			},
		},
		{
			Name:  "server",
			Usage: "Run the cothority server",
			Action: func(c *cli.Context) {
				runServer(c)
			},
			Flags: serverFlags,
		},
	}
	cliApp.Flags = serverFlags
	// default action
	cliApp.Action = func(c *cli.Context) error {
		runServer(c)
		return nil
	}

	cliApp.Run(os.Args)

}

func runServer(ctx *cli.Context) {
	// first check the options
	log.SetDebugVisible(ctx.Int("debug"))
	config := ctx.String("config")

	server.RunServer(config)
}
