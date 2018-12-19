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
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/blscosi/blscosi/check"
	_ "github.com/dedis/cothority/skipchain"
	_ "github.com/dedis/cothority/status/service"
	"github.com/dedis/kyber/util/encoding"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/cfgpath"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

const (
	// DefaultName is the name of the binary we produce and is used to create a directory
	// folder with this name
	DefaultName = "conode"

	// Version of this binary
	Version = "2.0"
)

var gitTag = ""

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = DefaultName
	cliApp.Usage = "run a cothority server"
	if gitTag == "" {
		cliApp.Version = Version
	} else {
		cliApp.Version = Version + "-" + gitTag
	}

	cliApp.Commands = []cli.Command{
		{
			Name:    "setup",
			Aliases: []string{"s"},
			Usage:   "Setup server configuration (interactive)",
			Action:  setup,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "non-interactive",
					Usage: "generate private.toml in non-interactive mode",
				},
				cli.IntFlag{
					Name:  "port",
					Usage: "which port to listen on",
					Value: 6879,
				},
				cli.StringFlag{
					Name:  "description",
					Usage: "the description to use",
					Value: "configured in non-interactive mode",
				},
			},
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
			Value: path.Join(cfgpath.GetConfigPath(DefaultName), app.DefaultServerConfig),
			Usage: "Configuration file of the server",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}

	// Do not allow conode to run when built in 32-bit mode.
	// The dedis/protobuf package is the origin of this limit.
	// Instead of getting the error later from protobuf and being
	// confused, just make it totally clear up-front.
	var i int
	iType := reflect.TypeOf(i)
	if iType.Size() < 8 {
		log.ErrFatal(errors.New("conode cannot run when built in 32-bit mode"))
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
	if tomlFileName == "" {
		log.Fatal("[-] Must give the roster file to check.")
	}
	return check.CothorityCheck(tomlFileName, c.Bool("detail"))
}

func setup(c *cli.Context) error {
	if c.String("config") != "" {
		log.Fatal("[-] Configuration file option cannot be used for the 'setup' command")
	}
	if c.String("debug") != "" {
		log.Fatal("[-] Debug option cannot be used for the 'setup' command")
	}

	if c.Bool("non-interactive") {
		port := c.Int("port")
		portStr := fmt.Sprintf("%v", port)

		serverBinding := network.NewAddress(network.TLS, net.JoinHostPort("", portStr))
		kp := key.NewKeyPair(cothority.Suite)

		pub, _ := encoding.PointToStringHex(cothority.Suite, kp.Public)
		priv, _ := encoding.ScalarToStringHex(cothority.Suite, kp.Private)

		conf := &app.CothorityConfig{
			Suite:       cothority.Suite.String(),
			Public:      pub,
			Private:     priv,
			Address:     serverBinding,
			Description: c.String("description"),
			Services:    app.GenerateServiceKeyPairs(),
		}

		out := path.Join(cfgpath.GetConfigPath(DefaultName), app.DefaultServerConfig)
		err := conf.Save(out)
		if err == nil {
			fmt.Fprintf(os.Stderr, "Wrote config file to %v\n", out)
		}

		// We are not going to write out the public.toml file here.
		// We don't because in the current use case for --non-interactive, which
		// is for containers to auto-generate configs on startup, the
		// roster (i.e. public IP addresses + public keys) will be generated
		// based on how Kubernetes does service discovery. Writing the public.toml
		// file based on the data we have here, would result in writing an invalid
		// public Address.

		// If we had written it, it would look like this:
		//  server := app.NewServerToml(cothority.Suite, kp.Public, conf.Address, conf.Description)
		//  group := app.NewGroupToml(server)
		//  group.Save(path.Join(dir, "public.toml"))

		return err
	}

	app.InteractiveConfig(cothority.Suite, DefaultName)
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
