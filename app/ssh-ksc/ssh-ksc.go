// This is the SSH-keystore client that allows to interact with any number
// of servers
package main

import (
	"os"

	"io/ioutil"
	"os/user"

	"strings"

	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/app/lib/server"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/oi"
	_ "github.com/dedis/cothority/services"
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
			Name:    "setup",
			Aliases: []string{"s"},
			Usage:   "setting up a new client",
			Action:  CmdSetup,
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
			Action:  check,
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
		os.Mkdir(c.String("config"), 0660)
		dbg.SetDebugVisible(c.Int("debug"))
		configFile = c.String("config") + "/config.bin"
		return nil
	}
	app.After = func(c *cli.Context) error {
		if clientApp != nil {
			err := SaveConfig()
			dbg.ErrFatal(err, "Error while creating config-file", configFile)
		}
		return nil
	}
	app.Run(os.Args)
}

func LoadConfig() error {
	file, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer file.Close()
	clientApp, err = identity.NewIdentityFromStream(file)
	oi.ErrFatal(err,
		"Problems reading config-file. Most probably you\n",
		"should start a new one by running with the 'setup'\n",
		"argument.")
	return nil
}

func SaveConfig() error {
	file, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer file.Close()
	err = clientApp.SaveToStream(file)
	return err
}

func CmdSetup(c *cli.Context) {
	name, err := os.Hostname()
	oi.ErrFatal(err, "Couldn't get hostname for naming")
	if c.Args().First() == "" {
		oi.Fatal("Group-file argument missing")
	}

	Setup(c.Args().First(), name, c.GlobalString("config-ssh")+"/id_rsa.pub")
}

func Setup(groupFile, hostname, pubFileName string) {
	groupFile = tildeToHome(groupFile)
	reader, err := os.Open(groupFile)
	oi.ErrFatal(err, "Didn't find group-file: ", groupFile)
	el, err := config.ReadGroupToml(reader)
	oi.ErrFatal(err, "Couldn't read group-file")
	pubFileName = tildeToHome(pubFileName)
	pubFile, err := os.Open(pubFileName)
	oi.ErrFatal(err, "Couldn't open public-ssh: ", pubFileName)
	pub, err := ioutil.ReadAll(pubFile)
	oi.ErrFatal(err, "Couldn't read public-ssh: ", pubFileName)

	clientApp = identity.NewIdentity(el, 2, hostname, string(pub))
	oi.ErrFatal(clientApp.CreateIdentity(), "Couldn't contact servers")
}

func clientDel(c *cli.Context) {
}

func update(c *cli.Context) {
	list(c)
}

func check(c *cli.Context) {
	server.CheckConfig(c.Args().First())
}

func confirm(c *cli.Context) {
	dbg.Print("Confirmed new config")
}

func list(c *cli.Context) {
}

func listNew(c *cli.Context) {
}

func tildeToHome(path string) string {
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		oi.ErrFatal(err)
		return usr.HomeDir + path[1:len(path)]
	}
	return path
}
