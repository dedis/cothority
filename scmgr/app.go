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

	"encoding/hex"

	"path"

	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet/network"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/urfave/cli.v1"
)

type Config struct {
	*skipchain.SkipBlockMap
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
					Name:  "height, he",
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
			Action:    add,
		},
		{
			Name:      "update",
			Usage:     "get latest valid block",
			Aliases:   []string{"u"},
			ArgsUsage: "skipchain-id",
			Action:    update,
		},
		{
			Name:   "list",
			Usage:  "lists all known skipblocks",
			Action: list,
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
	group := readGroup(c, 0)
	client := skipchain.NewClient()
	sb, cerr := client.CreateGenesis(group.Roster, c.Int("base"), c.Int("height"),
		skipchain.VerificationNone, nil, nil)
	if cerr != nil {
		log.Fatal("while creating the genesis-roster:", cerr)
	}
	log.Infof("Created new skipblock with id %x", sb.Hash)
	cfg := getConfigOrFail(c)
	cfg.Store(sb)
	log.ErrFatal(cfg.save(c))
	return nil
}

// Joins a given skipchain
func join(c *cli.Context) error {
	log.Info("Joining skipchain")
	if c.NArg() < 2 {
		return errors.New("Please give group-file and id of known block")
	}
	group := readGroup(c, 0)
	client := skipchain.NewClient()
	hash, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return err
	}
	gcr, cerr := client.GetUpdateChain(group.Roster, hash)
	if cerr != nil {
		return cerr
	}
	latest := gcr.Update[len(gcr.Update)-1]
	genesis := latest.GenesisID
	if genesis == nil {
		genesis = latest.Hash
	}
	log.Infof("Joined skipchain %x", genesis)
	cfg := getConfigOrFail(c)
	cfg.Store(latest)
	log.ErrFatal(cfg.save(c))
	return nil
}

// Returns the number of calls.
func add(c *cli.Context) error {
	log.Info("Adding a block with a new group")
	if c.NArg() < 2 {
		return errors.New("Please give group-file and id to add")
	}
	group := readGroup(c, 1)
	cfg := getConfigOrFail(c)
	sb := cfg.GetFuzzy(c.Args().First())
	if sb == nil {
		return errors.New("didn't find latest block - update first")
	}
	client := skipchain.NewClient()
	ssbr, cerr := client.StoreSkipBlock(sb, group.Roster, nil)
	if cerr != nil {
		return errors.New("while storing block: " + cerr.Error())
	}
	cfg.Store(ssbr.Latest)
	log.Infof("Added new block %x to chain %x", ssbr.Latest.Hash, ssbr.Latest.GenesisID)
	return nil
}

// Updates a block to the latest block
func update(c *cli.Context) error {
	log.Info("Updating block")
	if c.NArg() < 1 {
		return errors.New("please give block-id to update")
	}
	cfg := getConfigOrFail(c)

	sb := cfg.GetFuzzy(c.Args().First())
	if sb == nil {
		return errors.New("didn't find latest block in local store!")
	}
	client := skipchain.NewClient()
	guc, cerr := client.GetUpdateChain(sb.Roster, sb.Hash)
	if cerr != nil {
		return errors.New("while updating chain: " + cerr.Error())
	}
	if len(guc.Update) == 1 {
		log.Info("No new block available")
	} else {
		for _, b := range guc.Update[1:] {
			log.Infof("Adding new block %x to chain %x", b.Hash, b.GenesisID)
			cfg.Store(b)
		}
	}
	latest := guc.Update[len(guc.Update)-1]
	log.Infof("Latest block of %x is %x", latest.GenesisID, latest.Hash)
	log.ErrFatal(cfg.save(c))
	return nil
}

// List gets all known skipblocks
func list(c *cli.Context) error {
	cfg, err := LoadConfig(c)
	if err != nil {
		return errors.New("couldn't read config: " + err.Error())
	}
	if cfg.Length() == 0 {
		log.Info("Didn't find any blocks yet")
	} else {
		for _, sb := range cfg.SkipBlockMap.SkipBlocks {
			if sb.GenesisID.IsNull() {
				log.Infof("Genesis-block %x with roster %s",
					sb.Hash, sb.Roster.List)
			} else {
				log.Infof("Block %x with genesis %x and roster %s",
					sb.Hash, sb.GenesisID, sb.Roster.List)
			}
		}
	}
	return nil
}

func readGroup(c *cli.Context, pos int) *app.Group {
	if c.NArg() <= pos {
		log.Fatal("Please give the group-file as argument")
	}
	name := c.Args().Get(pos)
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

func LoadConfig(c *cli.Context) (*Config, error) {
	path := app.TildeToHome(c.GlobalString("config"))
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{SkipBlockMap: skipchain.NewSkipBlockMap()}, nil
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

func (cfg *Config) save(c *cli.Context) error {
	buf, err := network.Marshal(cfg)
	if err != nil {
		return err
	}
	file := app.TildeToHome(c.GlobalString("config"))
	path := path.Dir(file)
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		err := os.MkdirAll(path, 0770)
		if err != nil {
			return err
		}
	} else {
		return err
	}
	return ioutil.WriteFile(file, buf, 0660)
}
