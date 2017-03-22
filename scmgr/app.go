/*
* The skipchain-manager lets you create, modify and query skipchains
 */
package main

import (
	"os"

	"gopkg.in/dedis/onet.v1/app"

	"fmt"
	"io/ioutil"

	"errors"

	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/cothority_template/service"
	"github.com/dedis/onet/network"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/urfave/cli.v1"
)

type Config struct {
	Latest map[string]*skipchain.SkipBlock
}

func main() {
	network.RegisterMessage(&Config{})
	cliApp := cli.NewApp()
	cliApp.Name = "scmgr"
	cliApp.Usage = "Create, modify and query skipchains"
	cliApp.Version = "0.1"
	groupsDef := "the group-definition-file"
	cliApp.Commands = []cli.Command{
		{
			Name:      "create",
			Usage:     "make a new skipchain",
			Aliases:   []string{"c"},
			ArgsUsage: groupsDef,
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "base, b",
					Value: 2,
					Usage: "base for skipchains",
				},
				cli.IntFlag{
					Name:  "height, h",
					Value: 2,
					Usage: "maximum height of skipchain",
				},
			},
			Action: create,
		},
		{
			Name:      "join",
			Usage:     "join a skipchain and store it locally",
			Aliases:   []string{"j"},
			ArgsUsage: groupsDef + " skipchain-id",
			Action:    join,
		},
		{
			Name:      "add",
			Usage:     "add a block to a skipchain",
			Aliases:   []string{"a"},
			ArgsUsage: "skipchain-id " + groupsDef,
			Action:    blockAdd,
		},
		{
			Name:      "update",
			Usage:     "get latest valid block",
			Aliases:   []string{"u"},
			ArgsUsage: "skipchain-id",
			Action:    blockUpdate,
		},
	}
	cliApp.Flags = []cli.Flag{
		app.FlagDebug,
		cli.StringFlag{
			Name:  "config, c",
			Value: "~/.config/scmgr/config.bin",
			Usage: "path to config-file",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	cliApp.Run(os.Args)
}

// Creates a new skipchain with the given roster
func create(c *cli.Context) error {
	log.Info("Create skipchain")
	group := readGroup(c)
	client := skipchain.NewClient()
	sb, cerr := client.CreateGenesis(group, c.Int("base"), c.Int("height"),
		skipchain.VerificationStandard, nil, nil)
	if cerr != nil {
		log.Fatal("When asking the time:", cerr)
	}
	log.Infof("Created new skipblock with id %x", sb.Hash)
	cfg := getConfigOrFail(c)
	log.ErrFatal(cfg.addBlock(sb))
	log.ErrFatal(cfg.save(c))
	return nil
}

// Joins a given skipchain
func join(c *cli.Context) error {
	log.Info("Joining skipchain")
	group := readGroup(c)
	client := skipchain.NewClient()
	sb, cerr := client.CreateGenesis(group, c.Int("base"), c.Int("height"),
		skipchain.VerificationStandard, nil, nil)
	if cerr != nil {
		log.Fatal("When asking the time:", cerr)
	}
	log.Infof("Created new skipblock with id %x", sb.Hash)
	cfg := getConfigOrFail(c)
	log.ErrFatal(cfg.addBlock(sb))
	log.ErrFatal(cfg.save(c))
	return nil
}

// Returns the number of calls.
func blockAdd(c *cli.Context) error {
	log.Info("Counter command")
	group := readGroup(c)
	client := template.NewClient()
	counter, err := client.Count(group.Roster.RandomServerIdentity())
	if err != nil {
		log.Fatal("When asking for counter:", err)
	}
	log.Info("Number of requests:", counter)
	return nil
}

func readGroup(c *cli.Context) *app.Group {
	if c.NArg() != 1 {
		log.Fatal("Please give the group-file as argument")
	}
	name := c.Args().First()
	f, err := os.Open(name)
	log.ErrFatal(err, "Couldn't open group definition file")
	group, err := app.ReadGroupDescToml(f)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s",
			name)
	}
	return group
}

func getConfigOrFail(c *cli.Context) *Config {
	cfg, err := LoadConfig(c)
	log.ErrFatal(err)
	return cfg
}

func NewConfig() *Config {
	return &Config{
		Latest: map[string]*skipchain.SkipBlock{},
	}
}

func LoadConfig(c *cli.Context) (*Config, error) {
	path := app.TildeToHome(c.GlobalString("config"))
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewConfig(), nil
		} else {
			return nil, fmt.Errorf("Could not open file %s", path)
		}
	}
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	_, cfg, err := network.Unmarshal(f)
	if err != nil {
		return nil, err
	}
	return cfg.(*Config), err
}

func (cfg *Config) addBlock(sb *skipchain.SkipBlock) error {
	if cfg.Latest[sb.Hash] != nil {
		return errors.New("That skipblock already exists")
	}
	cfg.Latest[sb.Hash] = sb
	return nil
}

func (cfg *Config) getBlock(id skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
	if cfg.Latest[id] == nil {
		return errors.New("Don't know that skipblock")
	}
	return cfg.Latest[id], nil
}

func (cfg *Config) save(c *cli.Context) error {
	path := app.TildeToHome(c.GlobalString("config"))
	buf, err := network.Marshal(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, buf, 0660)
}
