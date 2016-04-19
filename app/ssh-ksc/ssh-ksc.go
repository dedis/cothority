// This is the SSH-keystore client that allows to interact with any number
// of servers
package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/services/sshks"
)

// Our clientApp configuration
var clientApp *sshks.ClientKS

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
				}, {
					Name:   "propose",
					Usage:  "propose to add a new config",
					Action: serverPropose,
				},
			},
		},
		{
			Name:    "clientRemove",
			Aliases: []string{"cr"},
			Usage:   "remove a client",
			Action:  clientDel,
		},
		{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "update to the latest list",
			Action:  update,
		},
		{
			Name:   "confirm",
			Usage:  "confirm a new configuration",
			Action: confirm,
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
		{
			Name:    "listNew",
			Aliases: []string{"ch"},
			Usage:   "list new servers and clients",
			Action:  listNew,
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
			Value: "~/.sshks",
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
		clientApp, err = sshks.ReadClientKS(configFile)
		if clientApp.This.SSHpub == "TestClient-" {
			clientApp.This.SSHpub += c.String("config")
		}
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
	err := clientApp.AddServerAddr(srvAddr)
	if err != nil {
		dbg.Print("Server refused to join - proposing new configuration")
	}
}

func serverDel(c *cli.Context) {
	srvAddr := c.Args().First()
	dbg.Print("Deleting server", srvAddr)
	err := clientApp.DelServerAddr(srvAddr)
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

func serverPropose(c *cli.Context) {
	err := clientApp.ServerProposeAddr(c.Args().First())
	if err != nil {
		dbg.Error(err)
	} else {
		dbg.Print("Proposed my config to the server - needs to be confirmed first.")
	}
}

func clientDel(c *cli.Context) {
	client := c.Args().First()
	dbg.Print("Deleting client with public key", client)
	err := clientApp.DelClient(clientApp.This)
	dbg.ErrFatal(err)
}

func update(c *cli.Context) {
	if len(clientApp.Config.Servers) == 0 {
		dbg.Print("No servers defined yet - use 'server add'")
		return
	}
	dbg.ErrFatal(clientApp.Update(nil))
	dbg.Print("Got latest configuration")
	list(c)
}

func confirm(c *cli.Context) {
	dbg.Print("Confirmed new config")
	err := clientApp.ConfirmNewConfig(nil)
	if err != nil {
		dbg.Print("Couldn't confirm:", err)
	}
	list(c)
}

func list(c *cli.Context) {
	clientApp.Config.List()
}

func listNew(c *cli.Context) {
	if clientApp.NewConfig != nil {
		dbg.Print("New config:")
		clientApp.NewConfig.List()
	}
}
