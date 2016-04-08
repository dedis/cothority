/*
Cothority is a general node that can be used for all available protocols.
*/
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/dedis/cothority/app"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	// Empty imports to have the init-functions called which should
	// register the protocol

	"github.com/codegangsta/cli"
	_ "github.com/dedis/cothority/protocols"
	"github.com/dedis/crypto/config"
)

// Main starts the host and will setup the protocol.
func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "Cosid server"
	cliApp.Usage = "Serve a cothority"
	cliApp.Version = "1.0"
	cliApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "config.toml",
			Usage: "config-file for the server",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 1,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}
	cliApp.Action = func(c *cli.Context) {
		config := c.String("config")
		dbg.SetDebugVisible(c.Int("debug"))
		dbg.Lvl1("Starting cothority daemon", cliApp.Version)
		// We're in standalone mode and only start the node
		host, err := sda.NewHostFromFile(config)
		// if the file does not exists
		if os.IsNotExist(err) {
			// then ask informations to create one
			host = createHost(config)
		} else if err != nil {
			dbg.Lvl1("Config file is invalid. (", err, ") Fix it or delete it and re-start to create a new one")
			os.Exit(1)
		}

		dbg.Lvl1("Starting Host: You can copy the following lines in a servers.toml file to use by app/cosi client:")
		serverToml := app.NewServerToml(host.Suite(), host.Entity.Public, host.Entity.Addresses...)
		groupToml := app.NewGroupToml(serverToml)
		fmt.Println(groupToml.String())

		host.Listen()
		host.StartProcessMessages()
		host.WaitForClose()
	}

	cliApp.Run(os.Args)

}

// createHost will ask for the public IP:PORT of the host we want to create.
// The IP:PORT pair *must* be accessible from the Internet as other Hosts will
// try to contact it.
func createHost(cfg string) *sda.Host {
	fmt.Println("Configuration file " + cfg + " does not exists")
	reader := bufio.NewReader(os.Stdin)
	var err error
	var str string
	// IP:PORT
	fmt.Println("Type the IP:PORT (ipv4) address of this host (accessible from Internet):")
	str, err = reader.ReadString('\n')
	str = strings.TrimSpace(str)
	_, _, errStr := net.SplitHostPort(str)
	if err != nil || errStr != nil {
		fmt.Println("[-] Error reading IP:PORT (", str, ") ", errStr, " => Abort")
		os.Exit(1)
	}

	ip := str

	// create the public / private keys
	kp := config.NewKeyPair(network.Suite)
	entity := network.NewEntity(kp.Public, ip)
	host := sda.NewHost(entity, kp.Secret)

	// write to the file
	if err = host.SaveToFile(cfg); err != nil {
		fmt.Println("Error writing config to file:", err, " => Abort.")
		os.Exit(1)
	}
	fmt.Println("Host written to file", str)
	return host
}
