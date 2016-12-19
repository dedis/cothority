package libcothority

import (
	"github.com/dedis/cothority/cosi/check"
	"github.com/dedis/onet/app/server"
	"gopkg.in/urfave/cli.v1"
)

// DefaultConfig is the name of the binary we produce and is used to create a directory
// folder with this name
const DefaultConfig = "cothorityd"

// CmdSetup is used to setup the cothority
var CmdSetup = cli.Command{
	Name:    "setup",
	Aliases: []string{"s"},
	Usage:   "Setup the configuration for the server (interactive)",
	Action: func(c *cli.Context) error {
		server.InteractiveConfig("cothorityd")
		return nil
	},
}

// CmdServer is used to start the server
var CmdServer = cli.Command{
	Name:  "server",
	Usage: "Run the cothority server",
	Action: func(c *cli.Context) {
		RunServer(c)
	},
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: server.GetDefaultConfigFile(DefaultConfig),
			Usage: "Configuration file of the server",
		},
	},
}

// CmdCheck is used to check all nodes in the group-file
var CmdCheck = cli.Command{
	Name:    "check",
	Aliases: []string{"c"},
	Usage:   "Check if the servers in the group definition are up and running",
	Action:  CheckConfig,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "g",
			Usage: "Cothority group definition file",
		},
		cli.BoolFlag{
			Name:  "detail, d",
			Usage: "give more detail in searching for errors",
		},
	},
}

// FlagDebug offers a debug-flag
var FlagDebug = cli.IntFlag{
	Name:  "debug, d",
	Value: 0,
	Usage: "debug-level: 1 for terse, 5 for maximal",
}

// RunServer starts the server
func RunServer(ctx *cli.Context) {
	// first check the options
	config := ctx.String("config")
	server.RunServer(config)
}

// CheckConfig contacts all servers and verifies if it receives a valid
// signature from each.
func CheckConfig(c *cli.Context) error {
	tomlFileName := c.String("g")
	return check.Config(tomlFileName, c.Bool("d"))
}
