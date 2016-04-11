// This is the SSH-keystore client that allows to interact with any number
// of servers
package main

import (
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/services/ssh-ks"
	"os"
)

// Our clientApp configuration
var clientApp *ssh_ks.ClientKS

// The config-file
var configFile string

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
		{
			Name:    "check",
			Aliases: []string{"ch"},
			Usage:   "check all servers",
			Action:  update,
		},
		{
			Name:    "list",
			Aliases: []string{"ch"},
			Usage:   "list servers and clients",
			Action:  list,
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
		dbg.SetDebugVisible(c.Int("debug"))
		var err error
		configFile = c.String("config") + "/config.bin"
		clientApp, err = ssh_ks.ReadClientKS(configFile)
		dbg.ErrFatal(err, "Couldn't read config-file")
		return nil
	}
	app.After = func(c *cli.Context) error {
		err := clientApp.Write(configFile)
		dbg.ErrFatal(err)
		return nil
	}
	app.Run(os.Args)
}

func serverAdd(c *cli.Context) {
	srvAddr := c.Args().First()
	dbg.Print("Contacting server", srvAddr)
	clientApp.ServerAdd(srvAddr)
}

func serverDel(c *cli.Context) {
	srvAddr := c.Args().First()
	dbg.Print("Deleting server", srvAddr)
	err := clientApp.ServerDel(srvAddr)
	dbg.ErrFatal(err)
	if len(clientApp.Config.Servers) == 0 {
		dbg.Print("Deleted last server")
	}
}

func serverCheck(c *cli.Context) {
	err := clientApp.ServerCheck()
	if err != nil {
		dbg.Error(err)
	} else {
		dbg.Print("Correctly checked servers")
	}
}

func clientAdd(c *cli.Context) {
	dbg.Print("Adding ourselves as client")
	err := clientApp.ClientAdd(clientApp.This)
	dbg.ErrFatal(err)
}

func clientDel(c *cli.Context) {
	dbg.Print("Deleting ourselves as client")
	err := clientApp.ClientDel(clientApp.This)
	dbg.ErrFatal(err)
}

func update(c *cli.Context) {
	dbg.ErrFatal(clientApp.Update(nil))
	dbg.Print("Got latest configuration")
	list(c)
}

func list(c *cli.Context) {
	clientApp.Config.List()
}
