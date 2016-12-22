// Cothorityd is the main binary for running a Cothority server.
// A Cothority server can participate in various distributed protocols using the
// *onet* library as a network and overlay library and the *dedis/crypto*
// library for all cryptographic primitives.
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

	"github.com/dedis/onet/log"
	"gopkg.in/urfave/cli.v1"

	_ "github.com/dedis/cothority/cosi/service"
	_ "github.com/dedis/cothority/guard/service"
	_ "github.com/dedis/cothority/identity"
	"github.com/dedis/cothority/libcothority"
	_ "github.com/dedis/cothority/skipchain"
	_ "github.com/dedis/cothority/status/service"
	"github.com/dedis/onet/app/server"
	// TEST_LINE - DON'T REMOVE
)

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
			Value: server.GetDefaultConfigFile(libcothority.DefaultConfig),
			Usage: "Configuration file of the server",
		},
		libcothority.FlagDebug,
	}

	cliApp.Commands = []cli.Command{
		libcothority.CmdSetup,
		libcothority.CmdServer,
		libcothority.CmdCheck,
	}
	cliApp.Flags = serverFlags

	// default action
	cliApp.Action = func(c *cli.Context) error {
		libcothority.RunServer(c)
		return nil
	}

	err := cliApp.Run(os.Args)
	log.ErrFatal(err)
}
