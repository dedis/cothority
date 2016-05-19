// This is the SSH-keystore client that allows to interact with any number
// of servers
package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/services/identity"
)

// Our clientApp configuration
var clientApp *identity.Identity

// The config-file
var configFile string

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore client"
	app.Usage = "Connects to a ssh-keystore-server and updates/changes information"
	app.Commands = []cli.Command{
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
		file, err := os.Open(configFile)
		defer file.Close()
		dbg.ErrFatal(err)
		clientApp, err = identity.NewIdentityFromStream(file)
		dbg.ErrFatal(err, "Couldn't read config-file")
		return nil
	}
	app.After = func(c *cli.Context) error {
		file, err := os.Create(configFile)
		dbg.ErrFatal(err)
		err = clientApp.SaveToStream(file)
		dbg.ErrFatal(err)
		return nil
	}
	app.Run(os.Args)
}

func clientDel(c *cli.Context) {
}

func update(c *cli.Context) {
	list(c)
}

func confirm(c *cli.Context) {
	dbg.Print("Confirmed new config")
}

func list(c *cli.Context) {
}

func listNew(c *cli.Context) {
}
