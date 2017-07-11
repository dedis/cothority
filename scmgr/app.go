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

	"bytes"
	"sort"

	"encoding/json"
	"path/filepath"
	"strings"

	"gopkg.in/dedis/cothority.v1/skipchain"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"gopkg.in/urfave/cli.v1"
)

type config struct {
	Sbm *skipchain.SkipBlockMap
}

type html struct {
	Data []byte
}

func main() {
	network.RegisterMessage(&config{})
	network.RegisterMessage(&html{})
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
				cli.StringFlag{
					Name:  "html",
					Usage: "URL of html-skipchain",
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
			Usage:     "add a new roster to a skipchain",
			Aliases:   []string{"a"},
			ArgsUsage: "skipchain-id " + groupsDef,
			Action:    add,
		},
		{
			Name:      "addWeb",
			Usage:     "add a web-site to a skipchain",
			Aliases:   []string{"a"},
			ArgsUsage: "skipchain-id page.html",
			Action:    addWeb,
		},
		{
			Name:      "update",
			Usage:     "get latest valid block",
			Aliases:   []string{"u"},
			ArgsUsage: "skipchain-id",
			Action:    update,
		},
		{
			Name:  "list",
			Usage: "handle list of skipblocks",
			Subcommands: []cli.Command{
				{
					Name:    "known",
					Aliases: []string{"k"},
					Usage:   "lists all known skipblocks",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "long, l",
							Usage: "give long id of blocks",
						},
					},
					Action: lsKnown,
				},
				{
					Name:      "index",
					Usage:     "create index-files for all known skipchains",
					ArgsUsage: "output path",
					Action:    lsIndex,
				},
				{
					Name:      "fetch",
					Usage:     "ask all known conodes for skipchains",
					ArgsUsage: "[group-file]",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "recursive, r",
							Usage: "recurse into other conodes",
						},
					},
					Action: lsFetch,
				},
			},
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
	data := []byte{}
	if address := c.String("html"); address != "" {
		if !strings.HasPrefix(address, "http") {
			log.Fatal("Please give http-address")
		}
		data = []byte(address)
	}
	sb, cerr := client.CreateGenesis(group.Roster, c.Int("base"), c.Int("height"),
		skipchain.VerificationStandard, &html{data}, nil)
	if cerr != nil {
		log.Fatal("while creating the genesis-roster:", cerr)
	}
	log.Infof("Created new skipblock with id %x", sb.Hash)
	cfg := getConfigOrFail(c)
	cfg.Sbm.Store(sb)
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
	cfg.Sbm.Store(latest)
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
	sb := cfg.Sbm.GetFuzzy(c.Args().First())
	if sb == nil {
		return errors.New("didn't find latest block - update first")
	}
	client := skipchain.NewClient()
	guc, cerr := client.GetUpdateChain(sb.Roster, sb.Hash)
	if cerr != nil {
		return cerr
	}
	latest := guc.Update[len(guc.Update)-1]
	ssbr, cerr := client.StoreSkipBlock(latest, group.Roster, nil)
	if cerr != nil {
		return errors.New("while storing block: " + cerr.Error())
	}
	cfg.Sbm.Store(ssbr.Latest)
	log.ErrFatal(cfg.save(c))
	log.Infof("Added new block %x to chain %x", ssbr.Latest.Hash, ssbr.Latest.GenesisID)
	return nil
}

// Adds a block with the page inside.
func addWeb(c *cli.Context) error {
	log.Info("Adding a block with a page")
	if c.NArg() < 2 {
		log.Fatal("Please give skipchain-id and html-file to save")
	}
	for i, s := range c.Args() {
		log.Info(i, s)
	}
	cfg := getConfigOrFail(c)
	sb := cfg.Sbm.GetFuzzy(c.Args().First())
	if sb == nil {
		return errors.New("didn't find latest block - update first")
	}
	client := skipchain.NewClient()
	guc, cerr := client.GetUpdateChain(sb.Roster, sb.Hash)
	if cerr != nil {
		return cerr
	}
	latest := guc.Update[len(guc.Update)-1]
	log.Info("Reading file", c.Args().Get(1))
	data, err := ioutil.ReadFile(c.Args().Get(1))
	log.ErrFatal(err)
	ssbr, cerr := client.StoreSkipBlock(latest, nil, &html{data})
	if cerr != nil {
		return errors.New("while storing block: " + cerr.Error())
	}
	cfg.Sbm.Store(ssbr.Latest)
	log.ErrFatal(cfg.save(c))
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

	sb := cfg.Sbm.GetFuzzy(c.Args().First())
	if sb == nil {
		return errors.New("didn't find latest block in local store")
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
			cfg.Sbm.Store(b)
		}
	}
	latest := guc.Update[len(guc.Update)-1]
	log.Infof("Latest block of %x is %x", latest.GenesisID, latest.Hash)
	log.ErrFatal(cfg.save(c))
	return nil
}

// lsKnown shows all known skipblocks
func lsKnown(c *cli.Context) error {
	cfg, err := loadConfig(c)
	if err != nil {
		return errors.New("couldn't read config: " + err.Error())
	}
	if cfg.Sbm.Length() == 0 {
		log.Info("Didn't find any blocks yet")
		return nil
	}
	for _, g := range cfg.getSortedGenesis() {
		short := !c.Bool("long")
		log.Info(g.Sprint(short))
		sub := sbli{}
		for _, sb := range cfg.Sbm.SkipBlocks {
			if sb.GenesisID.Equal(g.Hash) {
				sub = append(sub, sb)
			}
		}
		sort.Sort(sub)
		for _, sb := range sub {
			log.Info("  " + sb.Sprint(short))
		}
	}
	return nil
}

// lsIndex writes one index-file for every known skipchain and an index.html
// for all skiplchains.
func lsIndex(c *cli.Context) error {
	output := c.Args().First()
	if len(output) == 0 {
		return errors.New("Missing output path")
	}

	cleanHTMLFiles(output)

	cfg, err := loadConfig(c)
	if err != nil {
		return errors.New("couldn't read config: " + err.Error())
	}

	// Get the list of genesis block
	genesis := cfg.getSortedGenesis()

	// Build the json structure
	blocks := jsonBlockList{}
	blocks.Blocks = make([]jsonBlock, len(genesis))
	for i, g := range genesis {
		block := &blocks.Blocks[i]
		block.GenesisID = hex.EncodeToString(g.Hash)
		block.Servers = make([]string, len(g.Roster.List))
		block.Data = g.Data

		for j, server := range g.Roster.List {
			block.Servers[j] = server.Address.Host() + ":" + server.Address.Port()
		}

		// Write the genesis block file
		content, _ := json.Marshal(block)
		err := ioutil.WriteFile(filepath.Join(output, block.GenesisID+".html"), content, 0644)

		if err != nil {
			log.Info("Cannot write block-specific file")
		}
	}

	content, err := json.Marshal(blocks)
	if err != nil {
		log.Info("Cannot convert to json")
	}

	// Write the json into the index.html
	err = ioutil.WriteFile(filepath.Join(output, "index.html"), content, 0644)
	if err != nil {
		log.Info("Cannot write in the file")
	}

	return nil
}

func lsFetch(c *cli.Context) error {
	cfg := getConfigOrFail(c)
	rec := c.Bool("recursive")
	sisAll := map[network.ServerIdentityID]*network.ServerIdentity{}
	group := readGroup(c, 0)
	var sisNew []*network.ServerIdentity

	// Get ServerIdentities from all skipblocks
	for _, sb := range cfg.Sbm.SkipBlocks {
		sisNew = updateNewSIs(sb.Roster, sisNew, sisAll)
	}

	// Get ServerIdentities from the given group-file
	sisNew = updateNewSIs(group.Roster, sisNew, sisAll)

	log.Info("The following ips will be searched:")
	for _, si := range sisNew {
		log.Info(si.Address)
	}
	client := skipchain.NewClient()
	for len(sisNew) > 0 {
		si := sisNew[0]
		if len(sisNew) > 1 {
			sisNew = sisNew[1:]
		} else {
			sisNew = []*network.ServerIdentity{}
		}
		log.Info("si, sisNew:", si, sisNew)
		gasr, cerr := client.GetAllSkipchains(si)
		if cerr != nil {
			// Error is not fatal here - perhaps the node is down,
			// but we can continue anyway.
			log.Error(cerr)
			continue
		}
		for _, sb := range gasr.SkipChains {
			log.Infof("Found skipchain %x", sb.SkipChainID())
			cfg.Sbm.Store(sb)
			if rec {
				log.Info("Recursive fetch")
				sisNew = updateNewSIs(sb.Roster, sisNew, sisAll)
			}
		}
	}
	return cfg.save(c)
}

// Remove every file matching *.html in the given directory
func cleanHTMLFiles(dir string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".html") {
			err := os.Remove(filepath.Join(dir, f.Name()))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// JSON skipblock element to be written in the index.html file
type jsonBlock struct {
	GenesisID string
	Servers   []string
	Data      []byte
}

// JSON list of skipblocks element to be written in the index.html file
type jsonBlockList struct {
	Blocks []jsonBlock
}

// sbl is used to make a nice output with ordered list of geneis-skipblocks.
type sbl []*skipchain.SkipBlock

func (s sbl) Len() int {
	return len(s)
}
func (s sbl) Less(i, j int) bool {
	return bytes.Compare(s[i].Hash, s[j].Hash) < 0
}
func (s sbl) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// sbli is used to make a nice output with ordered list of skipblocks of a
// skipchain.
type sbli sbl

func (s sbli) Len() int {
	return len(s)
}
func (s sbli) Less(i, j int) bool {
	return s[i].Index < s[j].Index
}
func (s sbli) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
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

func getConfigOrFail(c *cli.Context) *config {
	cfg, err := loadConfig(c)
	log.ErrFatal(err)
	return cfg
}

func loadConfig(c *cli.Context) (*config, error) {
	path := app.TildeToHome(c.GlobalString("config"))
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &config{Sbm: skipchain.NewSkipBlockMap()}, nil
		}
		return nil, fmt.Errorf("Could not open file %s", path)
	}
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	_, cfg, err := network.Unmarshal(f)
	if err != nil {
		return nil, err
	}
	return cfg.(*config), err
}

func (cfg *config) save(c *cli.Context) error {
	buf, err := network.Marshal(cfg)
	if err != nil {
		return err
	}
	file := app.TildeToHome(c.GlobalString("config"))
	path := path.Dir(file)
	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(path, 0770)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return ioutil.WriteFile(file, buf, 0660)
}

func (cfg *config) getSortedGenesis() []*skipchain.SkipBlock {
	genesis := sbl{}
	for _, sb := range cfg.Sbm.SkipBlocks {
		if sb.Index == 0 {
			genesis = append(genesis, sb)
		}
	}
	sort.Sort(genesis)
	return genesis
}

func updateNewSIs(roster *onet.Roster, sisNew []*network.ServerIdentity,
	sisAll map[network.ServerIdentityID]*network.ServerIdentity) []*network.ServerIdentity {
	for _, si := range roster.List {
		if _, exists := sisAll[si.ID]; !exists {
			log.Info("Adding", si)
			sisNew = append(sisNew, si)
			sisAll[si.ID] = si
		}
	}
	return sisNew
}
