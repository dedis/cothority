// Conode is the main binary for running a Cothority server.
// A conode can participate in various distributed protocols using the
// *onet* library as a network and overlay library and the *kyber*
// library for all cryptographic primitives.
// Basically, you first need to setup a config file for the server by using:
//
//  ./conode setup
//
// Then you can launch the daemon with:
//
//  ./conode
//
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/ftcosi/check"
	_ "github.com/dedis/cothority/ftcosi/service"
	_ "github.com/dedis/cothority/identity"
	_ "github.com/dedis/cothority/skipchain"
	_ "github.com/dedis/cothority/status/service"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/cfgpath"
	"github.com/dedis/onet/log"
	"gopkg.in/urfave/cli.v1"
)

const (
	// DefaultName is the name of the binary we produce and is used to create a directory
	// folder with this name
	DefaultName = "conode"

	// Version of this binary
	Version = "2.0"
)

func main() {

	cliApp := cli.NewApp()
	cliApp.Name = "conode"
	cliApp.Usage = "run a cothority server"
	cliApp.Version = Version

	cliApp.Commands = []cli.Command{
		{
			Name:    "setup",
			Aliases: []string{"s"},
			Usage:   "Setup server configuration (interactive)",
			Action:  setup,
		},
		{
			Name:   "server",
			Usage:  "Start cothority server",
			Action: runServer,
		},
		{
			Name:      "check",
			Aliases:   []string{"c"},
			Usage:     "Check if the servers in the group definition are up and running",
			ArgsUsage: "Cothority group definition file",
			Action:    checkConfig,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "g",
					Usage: "Cothority group definition file",
				},
				cli.BoolFlag{
					Name:  "detail, l",
					Usage: "Do pairwise signing and show full addresses",
				},
			},
		},
		{
			Name:   "convert64",
			Usage:  "convert a base64 toml file to a hex toml file",
			Action: convert64,
		},
	}
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: path.Join(cfgpath.GetConfigPath("conode"), app.DefaultServerConfig),
			Usage: "Configuration file of the server",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}

	err := cliApp.Run(os.Args)
	log.ErrFatal(err)
}

func runServer(ctx *cli.Context) error {
	// first check the options
	config := ctx.GlobalString("config")
	app.RunServer(config)
	return nil
}

// checkConfig contacts all servers and verifies if it receives a valid
// signature from each.
func checkConfig(c *cli.Context) error {
	tomlFileName := c.String("g")
	if c.NArg() > 0 {
		tomlFileName = c.Args().First()
	}
	return check.Config(tomlFileName, c.Bool("detail"))
}

func setup(c *cli.Context) error {
	if c.String("config") != "" {
		log.Fatal("[-] Configuration file option cannot be used for the 'setup' command")
	}
	if c.String("debug") != "" {
		log.Fatal("[-] Debug option cannot be used for the 'setup' command")
	}
	app.InteractiveConfig(cothority.Suite, "conode")
	return nil
}

// Convert toml files from base64-encoded kyes to hex-encoded keys
func convert64(c *cli.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if strings.HasPrefix(line, "  Public = \"") {
			pub := line[12:]
			if pub[len(pub)-1] != '"' {
				// does not end in quote? error.
				log.Fatal("Expected line to end in quote, but it does not: ", line)
			}
			pub = pub[:len(pub)-1]
			data, err := base64.StdEncoding.DecodeString(pub)
			if err != nil {
				log.Fatal("line", lineNo, "error:", err)
			}
			fmt.Printf("  Public = \"%v\"\n", hex.EncodeToString(data))
		} else {
			fmt.Println(line)
		}
	}
	return nil
}
