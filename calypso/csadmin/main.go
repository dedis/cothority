package main

import (
	"encoding/hex"
	"errors"
	"math/rand"
	"os"
	"time"

	"go.dedis.ch/cothority/v3/calypso"
	"go.dedis.ch/onet/v3/app"

	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
	"gopkg.in/urfave/cli.v1"
)

var cmds = cli.Commands{
	{
		Name:      "authorize",
		Usage:     "store the byzcoin-id that should be trusted to create new LTS",
		Aliases:   []string{"a"},
		ArgsUsage: "private.toml",
		Action:    authorize,
	},
}

var cliApp = cli.NewApp()

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

var gitTag = "dev"

func init() {
	cliApp.Name = "csadmin"
	cliApp.Usage = "Handle the calypso service"
	cliApp.Version = gitTag
	cliApp.Commands = cmds
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		lib.ConfigPath = c.String("config")
		return nil
	}
}

func main() {
	rand.Seed(time.Now().Unix())
	err := cliApp.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func authorize(c *cli.Context) error {
	if c.NArg() < 2 {
		return errors.New("please give: private.toml byzcoin-id")
	}

	cfg, err := app.LoadCothority(c.Args().First())
	if err != nil {
		return err
	}
	si, err := cfg.GetServerIdentity()
	if err != nil {
		return err
	}

	bc, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return err
	}
	log.Infof("Contacting %s to authorize byzcoin %x", si.Address, bc)
	cl := calypso.NewClient(nil)
	return cl.Authorize(si, bc)
}
