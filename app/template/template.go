/*
* This is a template for creating an app. It only has one command which
* prints out the name of the app.
 */
package main

import (
	"os"

	"github.com/dedis/cothority/log"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "Template"
	app.Usage = "Used for building other apps."
	app.Version = "0.1"
	app.Commands = []cli.Command{
		{
			Name:      "main",
			Usage:     "main command",
			Aliases:   []string{"m"},
			ArgsUsage: "additional parameters",
			Action:    cmdMain,
		},
	}
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}
	app.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	app.Run(os.Args)

}

// Main command.
func cmdMain(c *cli.Context) error {
	log.Info("Main command")
	return nil
}
