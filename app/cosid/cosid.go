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

	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/cosi"
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
		cosiApp, err := cosi.ReadCosiApp(config)
		if err != nil {
			// then ask informations to create one
			cosiApp = createHost(config)
			dbg.ErrFatal(cosiApp.Write(config))
		}

		dbg.Lvl1("Starting Host: You can copy the following lines in a servers.toml file to use by app/cosi client:")
		cosiApp.PrintServer()
		cosiApp.Start()
	}

	cliApp.Run(os.Args)

}

// createHost will ask for the public IP:PORT of the host we want to create.
// The IP:PORT pair *must* be accessible from the Internet as other Hosts will
// try to contact it.
func createHost(cfg string) *cosi.CosiApp {
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
	return cosi.CreateCosiApp(ip)
}
