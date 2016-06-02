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
	"github.com/dedis/cothority/app/lib/ui"
	"github.com/dedis/cothority/lib/dbg"
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
			Name:      "ownerRemove",
			Aliases:   []string{"or"},
			Usage:     "remove an owner",
			ArgsUsage: "owner-name",
			Action:    ownerDel,
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
		configDir := tildeToHome(c.String("config"))
		os.Mkdir(configDir, 0660)
		dbg.SetDebugVisible(c.Int("debug"))
		configFile = configDir + "/config.bin"
		if err := loadConfig(); err != nil {
			ui.Error("Problems reading config-file. Most probably you\n",
				"should start a new one by running with the 'setup'\n",
				"argument.")
		}
		return nil
	}
	app.After = func(c *cli.Context) error {
		if clientApp != nil {
			err := saveConfig()
			ui.ErrFatal(err, "Error while creating config-file", configFile)
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
		ui.ErrFatal(err, "Couldn't get hostname for naming")
	}
	if c.Args().First() == "" {
		ui.Fatal("Group-file argument missing")
	}

	setup(c.Args().First(), name, c.GlobalString("config-ssh")+"/id_rsa.pub",
		c.String("add"))
}

func setup(groupFile, hostname, pubFileName, add string) {
	groupFile = tildeToHome(groupFile)
	reader, err := os.Open(groupFile)
	ui.ErrFatal(err, "Didn't find group-file: ", groupFile)
	el, err := config.ReadGroupToml(reader)
	reader.Close()
	ui.ErrFatal(err, "Couldn't read group-file")
	if len(el.List) == 0 {
		ui.Fatal("EntityList is empty")
	}

	pubFileName = tildeToHome(pubFileName)
	pubFile, err := os.Open(pubFileName)
	ui.ErrFatal(err, "Couldn't open public-ssh: ", pubFileName)
	pub, err := ioutil.ReadAll(pubFile)
	pubFile.Close()
	ui.ErrFatal(err, "Couldn't read public-ssh: ", pubFileName)
	clientApp = identity.NewIdentity(el, 2, hostname, string(pub))

	if add == "" {
		ui.ErrFatal(clientApp.CreateIdentity(), "Couldn't contact servers")
	} else {
		id, err := hex.DecodeString(add)
		ui.ErrFatal(err, "Couldn't convert id to hex")
		ui.ErrFatal(clientApp.AttachToIdentity(identity.ID(id)), "Couldn't attach")
		ui.Info("Proposed to attach to given identity - now you need to confirm it.")
	}
}

func ownerDel(c *cli.Context) {
	owner := c.Args().First()
	if _, exists := clientApp.Config.Owners[owner]; !exists {
		ui.Info("Owners available:")
		for o := range clientApp.Config.Owners {
			ui.Info(o)
		}
		return
	}
	prop := clientApp.Config.Copy()
	delete(prop.Owners, owner)
	ui.ErrFatal(clientApp.ConfigNewPropose(prop))
	ui.ErrFatal(clientApp.VoteProposed(true))
	update(c)
}

func update(c *cli.Context) {
	checkClientApp()
	ui.ErrFatal(clientApp.ConfigUpdate(), "Couldn't update the config")
	ui.ErrFatal(clientApp.ConfigNewCheck(), "Didn't get newest proposals")
	list(c)
}

func check(c *cli.Context) {
	groupFile := c.Args().First()
	if groupFile == "" {
		ui.Info("Taking default group-definition")
		ui.ErrFatal(server.CheckServers(&config.Group{EntityList: clientApp.Cothority}))
	} else {
		ui.ErrFatal(server.CheckConfig(c.Args().First()))
	}
}

func confirm(c *cli.Context) {
	checkClientApp()
	if clientApp.Proposed == nil {
		ui.Fatal("Didn't find a proposed config - check if one exists with 'listNew'.")
	}
	ui.ErrFatal(clientApp.VoteProposed(true))
	ui.Info("Confirmed new config")
	ui.ErrFatal(clientApp.ConfigUpdate())
	ui.ErrFatal(clientApp.ConfigNewCheck())
	list(c)
}

func list(c *cli.Context) {
	ui.Info("Account name:", clientApp.ManagerStr)
	ui.Infof("Identity-ID: %x", clientApp.ID)
	ui.Infof("Current config: %s", clientApp.Config)
}

func listNew(c *cli.Context) {
	if clientApp == nil {
		ui.Fatal("No configuration available. Please 'setup' first.")
	}
	ui.ErrFatal(clientApp.ConfigNewCheck(), "Couldn't update the config")
	list(c)
	if clientApp.Proposed != nil {
		ui.Infof("Proposed config: %s", clientApp.Proposed)
	}
}

func tildeToHome(path string) string {
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		ui.ErrFatal(err)
		return usr.HomeDir + path[1:len(path)]
	}
	return path
}

func checkClientApp() {
	if clientApp == nil {
		ui.Fatal("No configuration available. Please 'setup' first.")
	}
}
