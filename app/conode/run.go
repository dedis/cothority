package main

import (
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
)

var maxRounds = -1

func init() {
	command := cli.Command{
		Name:    "run",
		Aliases: []string{"r"},
		Usage:   "Runs the CoNode and connects it to the cothority tree as specified in the config file",
		Action: func(c *cli.Context) {
			Run(c.String("config"), c.String("key"))
		},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name: "key, k",
				Usage: "Basename of the files where reside the keys. If key = 'key'," +
					"then conode will search through 'key.pub' and 'key.priv'",
				Value: defaultKeyFile,
			},
			cli.StringFlag{
				Name:  "config, c",
				Usage: "Configuration file of the cothority tree",
				Value: defaultConfigFile,
			},
		},
	}
	registerCommand(command)
}

// Run will launch the conode server. It takes a config file and a key file
// First parse the key + config file and then run the actual server
func Run(configFile, key string) {
	var address string
	// Read the global config
	conf := &app.ConfigConode{}
	if err := app.ReadTomlConfig(conf, configFile); err != nil {
		dbg.Fatal("Could not read toml config:", err)
	}
	dbg.Lvl1("Configuration file read")
	// Read the private / public keys + binded address
	if sec, err := cliutils.ReadPrivKey(suite, namePriv(key)); err != nil {
		dbg.Fatal("Error reading private key file:", err)
	} else {
		conf.Secret = sec
	}
	if pub, addr, err := cliutils.ReadPubKey(suite, namePub(key)); err != nil {
		dbg.Fatal("Error reading public key file:", err)
	} else {
		conf.Public = pub
		address = addr
	}
	peer := conode.NewPeer(address, conf)
	peer.SetupConnections()
	peer.LoopRounds(RoundStatsType, maxRounds)
}
