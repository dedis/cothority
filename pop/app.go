package main

import (
	"os"

	"github.com/dedis/cothority/cosi/check"
	_ "github.com/dedis/cothority/cosi/protocol"
	_ "github.com/dedis/cothority/cosi/service"

	"fmt"
	"io/ioutil"

	"github.com/dedis/cothority/pop/service"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/onet.v1/app"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"gopkg.in/urfave/cli.v1"
)

func init() {
	network.RegisterMessage(Config{})
}

// Config represents either a manager or an attendee configuration.
type Config struct {
	// Index of the attendee in the final statement. If the index
	// is -1, then this pop holds an organizer.
	Index int
	// Private key of attendee or organizer, depending on value
	// of Index.
	Private abstract.Scalar
	// Public key of attendee or organizer, depending on value of
	// index.
	Public abstract.Point
	// Address of the linked conode.
	Address network.Address
	// Final statement of the party.
	Final *service.FinalStatement
	// config-file name
	name string
}

func main() {
	appCli := cli.NewApp()
	appCli.Name = "Proof-of-personhood party"
	appCli.Usage = "Handles party-creation, finalizing, pop-token creation, and verification"
	appCli.Version = "0.1"
	appCli.Commands = []cli.Command{}
	appCli.Commands = []cli.Command{
		{
			Name:      "check",
			Aliases:   []string{"c"},
			Usage:     "Check if the servers in the group definition are up and running",
			ArgsUsage: "group.toml",
			Action: func(c *cli.Context) error {
				return check.Config(c.Args().First(), false)
			},
		},
	}
	appCli.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug,d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config,c",
			Value: "~/.config/cothority/pop",
			Usage: "The configuration-directory of pop",
		},
	}
	appCli.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	appCli.Run(os.Args)
}

// newConfig tries to read the config and returns an organizer-
// config if it doesn't find anything.
func newConfig(fileConfig string) (*Config, error) {
	name := app.TildeToHome(fileConfig)
	if _, err := os.Stat(name); err != nil {
		kp := config.NewKeyPair(network.Suite)
		return &Config{
			Private: kp.Secret,
			Public:  kp.Public,
			Final: &service.FinalStatement{
				Attendees: []abstract.Point{},
				Signature: []byte{},
			},
			Index: -1,
			name:  name,
		}, nil
	}
	buf, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("couldn't read %s: %s - please remove it",
			name, err)
	}
	_, msg, err := network.Unmarshal(buf)
	if err != nil {
		return nil, fmt.Errorf("error while reading file %s: %s",
			name, err)
	}
	cfg, ok := msg.(*Config)
	if !ok {
		log.Fatal("Wrong data-structure in file", name)
	}
	cfg.name = name
	return cfg, nil
}

// write saves the config to the given file.
func (cfg *Config) write() {
	buf, err := network.Marshal(cfg)
	log.ErrFatal(err)
	log.ErrFatal(ioutil.WriteFile(cfg.name, buf, 0660))
}
