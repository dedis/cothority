// Conode is the main binary for running a Cothority server.
// A conode can participate in various distributed protocols using the
// *onet* library as a network and overlay library and the *dedis/crypto*
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
	"strings"

	"github.com/dedis/cothority"
	"github.com/dedis/onet/log"
	"gopkg.in/urfave/cli.v1"

	"github.com/dedis/cothority/cosi/check"
	_ "github.com/dedis/cothority/status/service"
	_ "github.com/dedis/onchain-secrets/service"
	"github.com/dedis/onet/app"
)

const (
	// DefaultName is the name of the binary we produce and is used to create a directory
	// folder with this name
	DefaultName = "conode"

	// Version of this binary
	Version = "1.1"
)

func main() {

	cliApp := cli.NewApp()
	cliApp.Name = "conode"
	cliApp.Usage = "run a cothority server"
	cliApp.Version = Version
	serverFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: app.GetDefaultConfigFile(DefaultName),
			Usage: "configuration file of the server",
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
			Usage:   "Setup server configuration (interactive)",
			Action: func(c *cli.Context) error {
				if c.String("config") != "" {
					log.Fatal("[-] Configuration file option cannot be used for the 'setup' command")
				}
				if c.String("debug") != "" {
					log.Fatal("[-] Debug option cannot be used for the 'setup' command")
				}
				app.InteractiveConfig("conode")
				return nil
			},
		},
		{
			Name:  "server",
			Usage: "Start cothority server",
			Action: func(c *cli.Context) {
				runServer(c)
			},
			Flags: serverFlags,
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
	cliApp.Flags = serverFlags
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}

	err := cliApp.Run(os.Args)
	log.ErrFatal(err)
}

func runServer(ctx *cli.Context) {
	// first check the options
	config := ctx.String("config")

	app.RunServer(config, cothority.Suite)
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
