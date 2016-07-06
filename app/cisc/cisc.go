package main

import (
	"os"

	"io/ioutil"

	"errors"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/app/lib/ui"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/services/identity"
	"gopkg.in/codegangsta/cli.v1"
)

/*
Cisc is the Cisc Identity SkipChain to store information in a skipchain and
being able to retrieve it.

This is only one part of the system - the other part being the cothority that
holds the skipchain and answers to requests from the cisc-binary.
*/

var configFile string
var clientApp *identity.Identity

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore client"
	app.Usage = "Connects to a ssh-keystore-server and updates/changes information"
	app.Commands = []cli.Command{
		{
			Name:  "id",
			Usage: "working on the identity",
			Subcommands: []cli.Command{
				{
					Name:      "create",
					Aliases:   []string{"cr"},
					Usage:     "start a new identity",
					ArgsUsage: "group-definition [id-name]",
					Action:    idCreate,
				},
				{
					Name:    "connect",
					Aliases: []string{"co"},
					Usage:   "connect to an existing identity",
					Action:  idConnect,
				},
				{
					Name:    "remove",
					Aliases: []string{"rm"},
					Usage:   "remove an identity",
					Action:  idRemove,
				},
				{
					Name:    "follow",
					Aliases: []string{"f"},
					Usage:   "follow an existing identity",
					Action:  idFollow,
				},
				{
					Name:    "check",
					Aliases: []string{"ch"},
					Usage:   "check the health of the cothority",
					Action:  idCheck,
				},
			},
		},
		{
			Name:  "data",
			Usage: "updating and voting on data",
			Subcommands: []cli.Command{
				{
					Name:    "update",
					Aliases: []string{"u"},
					Usage:   "fetch the latest data",
					Action:  dataUpdate,
				},
				{
					Name:    "list",
					Aliases: []string{"ls"},
					Usage:   "list existing data and proposed",
					Action:  dataList,
				},
				{
					Name:    "vote",
					Aliases: []string{"v"},
					Usage:   "vote on existing data",
					Action:  dataVote,
				},
			},
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
			Value: "~/.cisc",
			Usage: "The configuration-directory of cisc",
		},
		cli.StringFlag{
			Name:  "config-ssh, cs",
			Value: "~/.ssh",
			Usage: "The configuration-directory of the ssh-directory",
		},
	}
	app.Before = func(c *cli.Context) error {
		configDir := config.TildeToHome(c.String("config"))
		os.Mkdir(configDir, 0660)
		log.SetDebugVisible(c.Int("debug"))
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

// loadConfig will return nil if the config-file doesn't exist. It tries to
// load the file given in configFile.
func loadConfig() error {
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_, msg, err := network.UnmarshalRegistered(buf)
	if err != nil {
		return err
	}
	var ok bool
	clientApp, ok = msg.(*identity.Identity)
	if !ok {
		return errors.New("Wrong message-type in config-file")
	}
	return nil
}

func saveConfig() error {
	buf, err := network.MarshalRegisteredType(clientApp)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configFile, buf, 0660)
}

func idCreate(c *cli.Context) error {
	if c.NArg() == 0 {
		log.Fatal("Please give at least a group-definition")
	}
	gfile := c.Args().Get(0)
	gr, err := os.Open(gfile)
	log.ErrFatal(err)
	groups, err := config.ReadGroupDescToml(gr)
	gr.Close()
	if len(groups.Roster.List) == 0 {
		log.Fatal("No servers found in roster from", gfile)
	}
	name, err := os.Hostname()
	log.ErrFatal(err)
	if c.NArg() > 1 {
		name = c.Args().Get(1)
	}
	log.Info("Creating new blockchain-identity for", name)
	return nil
}
func idConnect(c *cli.Context) error {
	return nil
}
func idRemove(c *cli.Context) error {
	return nil
}
func idFollow(c *cli.Context) error {
	return nil
}
func idCheck(c *cli.Context) error {
	return nil
}

func dataUpdate(c *cli.Context) error {
	return nil
}
func dataList(c *cli.Context) error {
	return nil
}
func dataVote(c *cli.Context) error {
	return nil
}
