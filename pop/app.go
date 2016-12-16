/*
* This is a template for creating an app. It only has one command which
* prints out the name of the app.
 */
package main

import (
	"os"

	"github.com/dedis/cothority/cosi/check"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app/config"
	"github.com/dedis/onet/log"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore client"
	app.Usage = "Connects to a ssh-keystore-server and updates/changes information"
	app.Version = "0.3"
	app.Commands = []cli.Command{
		commandMgr,
		commandClient,
		{
			Name:      "check",
			Aliases:   []string{"c"},
			Usage:     "Check if the servers in the group definition are up and running",
			ArgsUsage: "group.toml",
			Action:    checkConfig,
		},
	}
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "~/.config/cothority/pop",
			Usage: "The configuration-directory of pop",
		},
	}
	app.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	app.Run(os.Args)
}

// links this pop to a cothority
func mgrLink(c *cli.Context) error {
	log.Info("Mgr: ")
	return nil
}

// sets up a configuration
func mgrConfig(c *cli.Context) error {
	log.Info("Mgr: ")
	return nil
}

// adds a public key to the list
func mgrPublic(c *cli.Context) error {
	log.Info("Mgr: ")
	return nil
}

// finalizes the statement
func mgrFinal(c *cli.Context) error {
	log.Info("Mgr: ")
	return nil
}

// verifies a signature and tag
func mgrVerify(c *cli.Context) error {
	log.Info("Mgr: ")
	return nil
}

// creates a new private/public pair
func clientCreate(c *cli.Context) error {
	log.Info("Client: create")
	return nil
}

// joins a poparty
func clientJoin(c *cli.Context) error {
	log.Info("Client: join")
	return nil
}

// signs a message + context
func clientSign(c *cli.Context) error {
	log.Info("Client: sign")
	return nil
}

func readGroup(c *cli.Context) *onet.Roster {
	if c.NArg() != 1 {
		log.Fatal("Please give the group-file as argument")
	}
	name := c.Args().First()
	f, err := os.Open(name)
	log.ErrFatal(err, "Couldn't open group definition file")
	group, err := config.ReadGroupDescToml(f)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s",
			name)
	}
	return group.Roster
}

// checkConfig contacts all servers and verifies if it receives a valid
// signature from each.
func checkConfig(c *cli.Context) error {
	return check.Config(c.Args().First())
}
