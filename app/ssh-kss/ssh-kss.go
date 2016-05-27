// This is the ssh-keystore-server part that listens for requests of keystore-clients
// and will sign these requests.
package main

import (
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/services/identity"

	"os"

	"os/user"
	"strings"

	"encoding/hex"

	"io/ioutil"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/oi"
)

type servers struct {
	IDs []*identity.Identity
}

var serverKS = &servers{}

var configFile string

func main() {
	network.RegisterMessageType(servers{})
	app := cli.NewApp()
	app.Name = "SSH keystore server"
	app.Usage = "Serves as a server to listen to requests"
	app.Commands = []cli.Command{
		{
			Name:      "setup",
			Aliases:   []string{"s"},
			Usage:     "setting up a new server",
			Action:    setup,
			ArgsUsage: "group-file identity-hash",
		},
		{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "update to the latest list",
			Action:  update,
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "lists all identities and their accounts",
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
			Value: "/etc/ssh-ks",
			Usage: "The configuration-file of ssh-keystore",
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
		if len(serverKS.IDs) > 0 {
			saveConfig()
		}
		return nil
	}
	app.Run(os.Args)
}

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
	serverKS = msg.(*servers)
	return nil
}

func saveConfig() {
	buf, err := network.MarshalRegisteredType(serverKS)
	oi.ErrFatal(err, "Couldn't marshal servers")
	oi.ErrFatal(ioutil.WriteFile(configFile, buf, 0660))
	return
}

func setup(c *cli.Context) {
	groupFile := tildeToHome(c.Args().Get(0))
	idStr := c.Args().Get(1)
	if groupFile == "" {
		oi.Fatal("Please indicate the group-file to use")
	}
	if idStr == "" {
		oi.Fatal("Please inidicate what ID to follow")
	}

	reader, err := os.Open(groupFile)
	oi.ErrFatal(err, "Didn't find group-file: ", groupFile)
	defer reader.Close()
	el, err := config.ReadGroupToml(reader)
	oi.ErrFatal(err, "Couldn't read group-file")
	if el == nil {
		oi.Fatal("Group-file didn't contain any entities")
	}

	id, err := hex.DecodeString(idStr)
	oi.ErrFatal(err, "Couldn't convert id to hex")
	iden, err := identity.NewIdentityFromCothority(el, id)
	oi.ErrFatal(err, "Couldn't get identity")
	serverKS.IDs = append(serverKS.IDs, iden)

	list(c)
}

func update(c *cli.Context) {
	for _, s := range serverKS.IDs {
		oi.ErrFatal(s.ConfigUpdate())
	}

	list(c)
}

func list(c *cli.Context) {
	for i, s := range serverKS.IDs {
		oi.Infof("Server %d: %s", i, s.Config)
	}
}

func tildeToHome(path string) string {
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		oi.ErrFatal(err)
		return usr.HomeDir + path[1:len(path)]
	}
	return path
}
