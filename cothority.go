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

	"github.com/dedis/onet/app/server"
	"github.com/dedis/onet/log"
	"gopkg.in/urfave/cli.v1"

	"github.com/dedis/cothority/cosi/check"
	_ "github.com/dedis/cothority/cosi/service"
	_ "github.com/dedis/cothority/guard/service"
	_ "github.com/dedis/cothority/identity"
	_ "github.com/dedis/cothority/skipchain"
	_ "github.com/dedis/cothority/status/service"
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
		{
			Name:    "check",
			Aliases: []string{"c"},
			Usage:   "Check if the servers in the group definition are up and running",
			Action:  checkConfig,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "g",
					Usage: "Cothority group definition file",
				},
			},
		},
	}
	cliApp.Flags = serverFlags
	// default action
	cliApp.Action = func(c *cli.Context) error {
		runServer(c)
		return nil
	}

	err := cliApp.Run(os.Args)
	log.ErrFatal(err)
}

func runServer(ctx *cli.Context) {
	// first check the options
	log.SetDebugVisible(ctx.Int("debug"))
	config := ctx.String("config")

	server.RunServer(config)
}

// checkConfig contacts all servers and verifies if it receives a valid
// signature from each.
func checkConfig(c *cli.Context) error {
	tomlFileName := c.String("g")
	return check.Config(tomlFileName)
}
