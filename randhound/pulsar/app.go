package main

import (
	"os"

	"github.com/dedis/cothority/randhound"
	"github.com/dedis/cothority/randhound/protocol"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "pulsar"
	cliApp.Usage = "request verifiable randomness from a collective authority"
	cliApp.Version = "0.1"
	groupsDef := "the group-definition-file"
	cliApp.Commands = []cli.Command{
		{
			Name:      "setup",
			Usage:     "Configure the collective authority for randomness generation",
			Aliases:   []string{"s"},
			ArgsUsage: groupsDef,
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "groups, g",
					Value: 1,
					Usage: "Number of groups to be used",
				},
				cli.IntFlag{
					Name:  "interval, i",
					Value: 30000,
					Usage: "Randomness generation interval (in ms)",
				},
				cli.StringFlag{
					Name:  "purpose, p",
					Value: "pulsar app test",
					Usage: "Purpose of the randomness",
				},
			},
			Action: cmdSetup,
		},
		{
			Name:      "random",
			Usage:     "Request and verify randomness from a collective authority",
			Aliases:   []string{"r"},
			ArgsUsage: groupsDef,
			Action:    cmdRandom,
		},
	}
	cliApp.Flags = []cli.Flag{
		app.FlagDebug,
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	cliApp.Run(os.Args)
}

// Configure the collective authority for randomness generation.
func cmdSetup(c *cli.Context) error {
	// log.Info("Setup command")
	group := readGroup(c)
	client := randhound.NewClient()
	_, err := client.Setup(group.Roster, c.Int("groups"), c.String("purpose"),
		c.Int("interval"))
	if err != nil {
		return err
	}
	log.Infof("cothority setup succeeded")
	return nil
}

// Request and verify randomness from the collective authority.
func cmdRandom(c *cli.Context) error {
	// log.Info("Random command")
	group := readGroup(c)
	client := randhound.NewClient()
	reply, err := client.Random(group.Roster)
	if err != nil {
		return err
	}

	// Verify randomness
	if err := protocol.Verify(network.Suite, reply.R, reply.T); err != nil {
		log.Fatal("verification: failed\n", err)
	}

	// Log collective randomness
	log.Infof("collective randomness: %x", reply.R)
	log.Infof("verification: ok")
	log.Infof("timestamp: %s", reply.T.Time)

	return nil
}

func readGroup(c *cli.Context) *app.Group {
	if c.NArg() != 1 {
		log.Fatal("please provide the group file as an argument")
	}
	name := c.Args().First()
	f, err := os.Open(name)
	log.ErrFatal(err, "couldn't open group definition file")
	group, err := app.ReadGroupDescToml(f)
	log.ErrFatal(err, "error while reading group definition file")
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "empty entity or invalid group defintion in: %s",
			name)
	}
	return group
}
