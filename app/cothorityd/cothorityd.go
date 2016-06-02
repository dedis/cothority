// Cothorityd is the main binary for running a Cothority server.
// A Cothority server can participate in various distributed protocols using the
// *cothority/lib/sda* library with the underlying *dedis/crypto* library.
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
	"fmt"
	"os"
	"os/user"
	"path"
	"runtime"

	c "github.com/dedis/cothority/lib/config"
	"github.com/dedis/cothority/lib/dbg"
	"gopkg.in/codegangsta/cli.v1"
	// Empty imports to have the init-functions called which should
	// register the protocol

	_ "github.com/dedis/cothority/protocols"
	_ "github.com/dedis/cothority/services"
)

// DefaultName is the name of the binary we produce and is used to create a directory
// folder with this name
const DefaultName = "cothorityd"

// DefaultServerConfig is the default name of a server configuration file
const DefaultServerConfig = "config.toml"

// DefaultGroupFile is the default name of a group definition file
const DefaultGroupFile = "group.toml"

// Version of this binary
const Version = "1.1"

func main() {

	cliApp := cli.NewApp()
	cliApp.Name = "Cothorityd server"
	cliApp.Usage = "Serve a cothority"
	cliApp.Version = Version
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

	cliApp.Commands = []cli.Command{
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
	dbg.SetDebugVisible(ctx.Int("debug"))
	config := ctx.String("config")

	if _, err := os.Stat(config); os.IsNotExist(err) {
		dbg.Fatalf("[-] Configuration file does not exists. %s", config)
	}
	// Let's read the config
	_, host, err := c.ParseCothorityd(config)
	if err != nil {
		dbg.Fatal("Couldn't parse config:", err)
	}
	host.ListenAndBind()
	host.StartProcessMessages()
	host.WaitForClose()

}

func getDefaultConfigFile() string {
	u, err := user.Current()
	// can't get the user dir, so fallback to current working dir
	if err != nil {
		fmt.Print("[-] Could not get your home's directory. Switching back to current dir.")
		if curr, err := os.Getwd(); err != nil {
			stderrExit("[-] Impossible to get the current directory. %v", err)
		} else {
			return path.Join(curr, DefaultServerConfig)
		}
	}
	// let's try to stick to usual OS folders
	switch runtime.GOOS {
	case "darwin":
		return path.Join(u.HomeDir, "Library", DefaultName, DefaultServerConfig)
	default:
		return path.Join(u.HomeDir, ".config", DefaultName, DefaultServerConfig)
		// TODO WIndows ? FreeBSD ?
	}
}
