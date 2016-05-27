// This is the SSH-keystore client that allows to interact with any number
// of servers
package main

import (
	"os"

	"io/ioutil"
	"os/user"

	"strings"

	"encoding/hex"

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
			Name:      "setup",
			Aliases:   []string{"s"},
			Usage:     "setting up a new client",
			Action:    cmdSetup,
			ArgsUsage: "group-file",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "add, a",
					Usage: "adding to an existing identity",
				},
				cli.StringFlag{
					Name:  "name, n",
					Usage: "giving a name for this account",
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
			Action:  check,
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "list servers and clients",
			Action:  list,
		},
		{
			Name:    "listNew",
			Aliases: []string{"ln"},
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
		if err := loadConfig(); err != nil {
			oi.Error("Problems reading config-file. Most probably you\n",
				"should start a new one by running with the 'setup'\n",
				"argument.")
		}
		return nil
	}
	app.After = func(c *cli.Context) error {
		if clientApp != nil {
			err := saveConfig()
			dbg.ErrFatal(err, "Error while creating config-file", configFile)
		}
		return nil
	}
	app.Run(os.Args)
}

func loadConfig() error {
	file, err := os.Open(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	clientApp, err = identity.NewIdentityFromStream(file)
	if err != nil {
		return err
	}
	return nil
}

func saveConfig() error {
	file, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer file.Close()
	err = clientApp.SaveToStream(file)
	return err
}

func cmdSetup(c *cli.Context) {
	name, err := os.Hostname()
	if c.String("name") != "" {
		name = c.String("name")
	} else {
		oi.ErrFatal(err, "Couldn't get hostname for naming")
	}
	if c.Args().First() == "" {
		oi.Fatal("Group-file argument missing")
	}

	setup(c.Args().First(), name, c.GlobalString("config-ssh")+"/id_rsa.pub",
		c.String("add"))
}

func setup(groupFile, hostname, pubFileName, add string) {
	groupFile = tildeToHome(groupFile)
	reader, err := os.Open(groupFile)
	oi.ErrFatal(err, "Didn't find group-file: ", groupFile)
	defer reader.Close()
	el, err := config.ReadGroupToml(reader)
	oi.ErrFatal(err, "Couldn't read group-file")

	pubFileName = tildeToHome(pubFileName)
	pubFile, err := os.Open(pubFileName)
	oi.ErrFatal(err, "Couldn't open public-ssh: ", pubFileName)
	defer pubFile.Close()
	pub, err := ioutil.ReadAll(pubFile)
	oi.ErrFatal(err, "Couldn't read public-ssh: ", pubFileName)
	clientApp = identity.NewIdentity(el, 2, hostname, string(pub))

	if add == "" {
		oi.ErrFatal(clientApp.CreateIdentity(), "Couldn't contact servers")
	} else {
		id, err := hex.DecodeString(add)
		oi.ErrFatal(err, "Couldn't convert id to hex")
		oi.ErrFatal(clientApp.AttachToIdentity(identity.ID(id)), "Couldn't attach")
		oi.Info("Proposed to attach to given identity - now you need to confirm it.")
	}
}

func clientDel(c *cli.Context) {
}

func update(c *cli.Context) {
	checkClientApp()
	oi.ErrFatal(clientApp.ConfigUpdate(), "Couldn't update the config")
	oi.ErrFatal(clientApp.ConfigNewCheck(), "Didn't get newest proposals")
	list(c)
}

func check(c *cli.Context) {
	oi.ErrFatal(server.CheckConfig(c.Args().First()))
}

func confirm(c *cli.Context) {
	checkClientApp()
	if clientApp.Proposed == nil {
		oi.Fatal("Didn't find a proposed config - check if one exists with 'listNew'.")
	}
	oi.ErrFatal(clientApp.VoteProposed(true))
	oi.Info("Confirmed new config")
	oi.ErrFatal(clientApp.ConfigUpdate())
	oi.ErrFatal(clientApp.ConfigNewCheck())
	list(c)
}

func list(c *cli.Context) {
	oi.Info("Account name:", clientApp.ManagerStr)
	oi.Infof("Identity-ID: %x", clientApp.ID)
	oi.Infof("Current config: %s", clientApp.Config)
	if clientApp.Proposed != nil {
		oi.Infof("Proposed config: %s", clientApp.Proposed)
	}
}

func listNew(c *cli.Context) {
	if clientApp == nil {
		oi.Fatal("No configuration available. Please 'setup' first.")
	}
	oi.ErrFatal(clientApp.ConfigNewCheck(), "Couldn't update the config")
	list(c)
}

func tildeToHome(path string) string {
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		oi.ErrFatal(err)
		return usr.HomeDir + path[1:len(path)]
	}
	return path
}

func checkClientApp() {
	if clientApp == nil {
		oi.Fatal("No configuration available. Please 'setup' first.")
	}
}
