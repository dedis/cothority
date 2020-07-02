package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	cli "github.com/urfave/cli"
	"go.dedis.ch/cothority/v3/bevm"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

func init() {
	network.RegisterMessages(&darc.Darc{}, &darc.Identity{}, &darc.Signer{})
}

var cmds = cli.Commands{
	{
		Name:      "spawn",
		Usage:     "create a BEvm instance",
		Aliases:   []string{"s"},
		ArgsUsage: "",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "bc",
				EnvVar:   "BC",
				Usage:    "the ByzCoin config to use (required)",
				Required: true,
			},
			cli.StringFlag{
				Name: "darc",
				Usage: "DARC with the right to spawn a BEvm contract " +
					"(default is the admin DARC)",
			},
			cli.StringFlag{
				Name: "sign",
				Usage: "public key of the signing entity (default is " +
					"the admin public key)",
			},
			cli.StringFlag{
				Name:  "outID",
				Usage: "output file for the BEvm ID (optional)",
			},
		},
		Action: spawn,
	},
	{
		Name:      "delete",
		Usage:     "delete a BEvm instance",
		Aliases:   []string{"s"},
		ArgsUsage: "",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:     "bc",
				EnvVar:   "BC",
				Usage:    "the ByzCoin config to use (required)",
				Required: true,
			},
			cli.StringFlag{
				Name: "sign",
				Usage: "public key of the signing entity (default is " +
					"the admin public key)",
			},
			cli.StringFlag{
				Name:  "bevmID, i",
				Usage: "BEvm instance ID to delete",
			},
		},
		Action: delete,
	},
}

var cliApp = cli.NewApp()

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

var gitTag = "dev"

func init() {
	cliApp.Name = "bevmadmin"
	cliApp.Usage = "Manage BEvm instances."
	cliApp.Version = gitTag
	cliApp.Commands = cmds
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:   "config, c",
			EnvVar: "BC_CONFIG",
			Value:  getDataPath(cliApp.Name),
			Usage:  "path to configuration-directory",
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

func spawn(c *cli.Context) error {
	bcFile := c.String("bc")

	cfg, cl, err := lib.LoadConfig(bcFile)
	if err != nil {
		return fmt.Errorf("failed to load ByzCoin config: %v", err)
	}

	var signer *darc.Signer
	signerStr := c.String("sign")
	if signerStr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(signerStr)
	}
	if err != nil {
		return fmt.Errorf("failed to load signer key: %v", err)
	}

	var darc *darc.Darc
	darcStr := c.String("darc")
	if darcStr == "" {
		darcStr = cfg.AdminDarc.GetIdentityString()
	}
	darc, err = lib.GetDarcByString(cl, darcStr)
	if err != nil {
		return fmt.Errorf("failed to load DARC data: %v", err)
	}

	bevmInstID, err := bevm.NewBEvm(cl, *signer, darc)
	if err != nil {
		return fmt.Errorf("failed to spawn new BEvm instance: %v", err)
	}

	_, err = fmt.Fprintf(c.App.Writer, "Created BEvm instance with ID: %s\n",
		bevmInstID)
	if err != nil {
		return fmt.Errorf("failed to write report msg: %v", err)
	}

	// Save ID in file if provided
	outFile := c.String("outID")
	if outFile != "" {
		err = ioutil.WriteFile(outFile, []byte(bevmInstID.String()), 0644)
		if err != nil {
			return fmt.Errorf("failed to write BEvm ID to file: %v", err)
		}
	}

	return nil
}

func delete(c *cli.Context) error {
	bcFile := c.String("bc")

	cfg, cl, err := lib.LoadConfig(bcFile)
	if err != nil {
		return fmt.Errorf("failed to load ByzCoin config: %v", err)
	}

	var signer *darc.Signer
	signerStr := c.String("sign")
	if signerStr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(signerStr)
	}
	if err != nil {
		return fmt.Errorf("failed to load signer key: %v", err)
	}

	bevmIDStr := c.String("bevmID")

	bevmID, err := hex.DecodeString(bevmIDStr)
	if err != nil {
		return fmt.Errorf("invalid BEvm ID provided: %v", err)
	}
	bevmClient, err := bevm.NewClient(cl, *signer,
		byzcoin.NewInstanceID(bevmID))
	if err != nil {
		return fmt.Errorf("failed to retrieve BEvm client "+
			"instance: %v", err)
	}

	err = bevmClient.Delete()
	if err != nil {
		return fmt.Errorf("failed to delete BEvm instance: %v", err)
	}

	_, err = fmt.Fprintf(c.App.Writer, "Delete BEvm instance with ID: %s\n",
		bevmIDStr)
	if err != nil {
		return fmt.Errorf("failed to write report msg: %v", err)
	}

	return nil
}
