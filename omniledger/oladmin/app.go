/*
* This is a template for creating an app. It only has one command which
* prints out the name of the app.
 */
package main

import (
	"errors"
	"os"

	"github.com/dedis/onet/log"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "oladmin"
	cliApp.Usage = "Handles sharding of transactions"
	cliApp.Version = "0.1"
	cliApp.Commands = []cli.Command{
		{
			Name:      "create",
			Usage:     "creates the sharding",
			ArgsUsage: "the roster file",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "shards",
					Usage: "the number of shards on which the ledger will be partitioned",
				},
				cli.IntFlag{
					Name:  "epoch",
					Usage: "the epoch-size in number of blocks",
				},
			},
			Action: createSharding,
		},
		{
			Name:      "evolve",
			Usage:     "evolves a shard",
			ArgsUsage: "the omniledger config, the key config and the roster files",
			Action:    evolveShard,
		},
		{
			Name:      "newepoch",
			Usage:     "starts a new epoch",
			ArgsUsage: "the omniledger config and the key config files",
			Action:    newEpoch,
		},
	}
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	log.ErrFatal(cliApp.Run(os.Args))
}

// TODO: Finish the function
// creates two files (ol.cfg, key.cfg), the same way as bcadmin does
func createSharding(c *cli.Context) error {
	sn := c.Int("shards")
	if sn == 0 {
		return errors.New(`--shards flag is required 
			and its value must be greater than 0`)
	}

	es := c.Int("epoch")
	if es == 0 {
		return errors.New(`--epoch flag is required 
			and its value must be greather than 0`)
	}

	if c.NArg() < 5 { // NArg() counts the flags and their value as arguments
		return errors.New("Not enough arguments (1 required)")
	}
	fp := c.Args().First()

	// Open the roster file and handle its content

	return nil
}

// TODO: Finish function
func evolveShard(c *cli.Context) error {
	if c.NArg() < 3 {
		return errors.New("Not enough arguments (3 required")
	}

	olFP := c.Args().Get(0)
	keyFP := c.Args().Get(1)
	rosterFP := c.Args().Get(2)

	return nil
}

// TODO: Finish function
func newEpoch(c *cli.Context) error {
	if c.NArg() < 2 {
		return errors.New("Not enough arguments (2 required")
	}

	olFP := c.Args().Get(0)
	keyFP := c.Args().Get(1)

	return nil
}

// TODO: Finish function
func darc(c *cli.Context) error {

	return nil
}
