// This is the ssh-keystore-server part that listens for requests of keystore-clients
// and will sign these requests.
package main

import (
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/services/identity"

	"os"
)

// Server holds all identities
type Server struct {
	Identities []*identity.Identity
}

var serverKS *Server

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore server"
	app.Usage = "Serves as a server to listen to requests"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 1,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "/etc/ssh-ks",
			Usage: "The configuration-file of ssh-keystore",
		},
	}
	app.Action = func(c *cli.Context) {
		dbg.SetDebugVisible(c.Int("debug"))
		file := c.String("config")
		var err error
		serverKS, err = ReadIdentities(file)
		if err != nil {
			dbg.Fatal("Couldn't read identities")
		}
	}
	app.Run(os.Args)
}

// ReadIdentities goes through all files in ~/.ssh/*.id
func ReadIdentities(file string) (*Server, error) {
	return nil, nil
}
