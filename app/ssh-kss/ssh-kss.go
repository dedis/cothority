// This is the ssh-keystore-server part that listens for requests of keystore-clients
// and will sign these requests.
package main

import ()
import (
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"os"
)

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
			Value: "~/.ssh-ks",
			Usage: "The configuration-directory of ssh-keystore",
		},
		cli.StringFlag{
			Name:  "config-ssh, cs",
			Value: "~/.ssh",
			Usage: "The configuration-directory of the ssh-directory",
		},
		cli.StringFlag{
			Name:  "config-sshd, csd",
			Value: "/etc/ssh/ssh_host_rsa_key.pub",
			Usage: "SSH-daemon public key",
		},
	}
	app.Before = func(c *cli.Context) {
		dbg.Print(c.Int("debug"))
		dbg.SetDebugVisible(c.Int("debug"))
	}
	app.Run(os.Args)
}

func serverAdd(c *cli.Context)   {}
func serverDel(c *cli.Context)   {}
func serverCheck(c *cli.Context) {}
func clientAdd(c *cli.Context)   {}
func clientDel(c *cli.Context)   {}
func update(c *cli.Context)      {}
