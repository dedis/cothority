// This is the SSH-keystore client that allows to interact with any number
// of servers
package main

import (
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/ssh-ks"
	"os"
)

func init() {
	network.RegisterMessageType(ssh_ks.Config{})
}

var config *ssh_ks.Config

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore client"
	app.Usage = "Connects to a ssh-keystore-server and updates/changes information"
	app.Commands = []cli.Command{
		{
			Name:    "server",
			Aliases: []string{"s"},
			Usage:   "handle servers",
			Subcommands: []cli.Command{
				{
					Name:   "add",
					Usage:  "add a server",
					Action: serverAdd,
				}, {
					Name:   "del",
					Usage:  "deletes a server",
					Action: serverDel,
				}, {
					Name:   "check",
					Usage:  "check servers",
					Action: serverCheck,
				},
			},
		},
		{
			Name:    "client",
			Aliases: []string{"c"},
			Usage:   "handle clients",
			Subcommands: []cli.Command{
				{
					Name:   "add",
					Usage:  "add a client",
					Action: clientAdd,
				}, {
					Name:   "del",
					Usage:  "delete a client",
					Action: clientDel,
				},
			},
		},
		{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "update to the latest list",
			Action:  update,
		},
	}
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
	}
	app.Before = func(c *cli.Context) error {
		dbg.Print(c.Int("debug"))
		dbg.SetDebugVisible(c.Int("debug"))
		ssh_ks.ReadConfig(c.String("config") + "/config.bin")
		return nil
	}
	app.Run(os.Args)
}

func serverAdd(c *cli.Context) {

	// Check server
	config.AddServer()
}
func serverDel(c *cli.Context)   {}
func serverCheck(c *cli.Context) {}
func clientAdd(c *cli.Context)   {}
func clientDel(c *cli.Context)   {}
func update(c *cli.Context)      {}
