/*
* The skipchain-manager lets you create, modify and query skipchains
 */
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/util/encoding"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

type config struct {
	// The database holding all skipblocks
	Db *skipchain.SkipBlockDB
	// Values holds the different configuration values needed for scmgr
	Values *values
}

type values struct {
	Link map[string]*link
}

type link struct {
	Private kyber.Scalar
	Address network.Address
	Conode  *network.ServerIdentity
}

func main() {
	network.RegisterMessages(&config{}, &values{})
	cliApp := cli.NewApp()
	cliApp.Name = "scmgr"
	cliApp.Usage = "Create, modify and query skipchains"
	cliApp.Version = "0.2"
	cliApp.Commands = getCommands()
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
	log.ErrFatal(cliApp.Run(os.Args))
}

// adminLink tries to store our public key in the conode
func adminLink(c *cli.Context) error {
	if pin := c.String("pin"); pin != "" {
		return errors.New("pin entry not implemented yet")
	}
	private := c.String("private")
	if private == "" {
		return errors.New("give either pin or private.toml of conode")
	}
	var remote struct {
		Private string
		Public  string
		Address network.Address
	}
	_, err := toml.DecodeFile(private, &remote)
	if err != nil {
		return errors.New("error while reading private.toml: " + err.Error())
	}
	pkey, err := encoding.StringHexToScalar(skipchain.Suite, remote.Private)
	if err != nil {
		return errors.New("couldn't decode private key: " + err.Error())
	}
	pubkey, err := encoding.StringHexToPoint(skipchain.Suite, remote.Public)
	if err != nil {
		return errors.New("couldn't decode public key: " + err.Error())
	}
	cfg := getConfigOrFail(c)
	si := network.NewServerIdentity(pubkey, remote.Address)
	cfg.Values.Link[si.Public.String()] = &link{
		Private: pkey,
		Address: remote.Address,
		Conode:  si,
	}
	cerr := skipchain.NewClient().CreateLinkPrivate(si, pkey, skipchain.Suite.Point().Mul(pkey, nil))
	if cerr != nil {
		log.Error(cerr)
		return cerr
	}
	log.Lvl1("Correctly linked with", remote.Address)
	return cfg.save(c)
}
func adminUnlink(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("please give IP:Port of the service to unlink")
	}
	cfg := getConfigOrFail(c)
	link, err := findLinkFromAddress(cfg, c.Args().First())
	if err != nil {
		return err
	}
	cerr := skipchain.NewClient().Unlink(link.Conode, link.Private)
	if cerr != nil {
		return errors.New("couldn't unlink:" + cerr.Error())
	}
	log.Info("Successfull unlinked with", link.Conode)
	return nil
}
func adminFollow(c *cli.Context) error {
	cfg := getConfigOrFail(c)
	if c.NArg() == 0 {
		return errors.New("please give one of the following: (-id ID|-search ID [-any]|-lookup IP:Port:ID [-any]) conode")
	}
	link, err := findLinkFromAddress(cfg, c.Args().First())
	if err != nil {
		return errors.New("couldn't parse node-address or not linked yet: " + err.Error())
	}
	client := skipchain.NewClient()
	if idStr := c.String("id"); idStr != "" {
		scid, err := hex.DecodeString(idStr)
		if err != nil {
			return errors.New("invalid skipchain-id: " + err.Error())
		}
		cerr := client.AddFollow(link.Conode, link.Private, scid, skipchain.FollowID,
			skipchain.NewChainNone, "")
		if cerr != nil {
			return errors.New("couldn't add this block as chain-follower: " + cerr.Error())
		}
	} else if idStr := c.String("search"); idStr != "" {
		scid, err := hex.DecodeString(idStr)
		if err != nil {
			return errors.New("invalid skipchain-id: " + err.Error())
		}
		nc := skipchain.NewChainStrictNodes
		if c.Bool("any") {
			nc = skipchain.NewChainAnyNode
		}
		cerr := client.AddFollow(link.Conode, link.Private, scid, skipchain.FollowSearch,
			nc, "")
		if cerr != nil {
			return errors.New("couldn't find this block in search: " + cerr.Error())
		}
	} else if scURL := c.String("lookup"); scURL != "" {
		addr := strings.Split(scURL, ":")
		if len(addr) != 3 {
			return errors.New("please give conodeIP:conodePort:skipchain-id")
		}
		scid, err := hex.DecodeString(addr[2])
		if err != nil {
			return errors.New("invalid skipchain-id: " + err.Error())
		}
		nc := skipchain.NewChainStrictNodes
		if c.Bool("any") {
			nc = skipchain.NewChainAnyNode
		}
		cerr := client.AddFollow(link.Conode, link.Private, scid, skipchain.FollowLookup,
			nc, addr[0]+":"+addr[1])
		if cerr != nil {
			return errors.New("couldn't lookup this block: " + cerr.Error())
		}
	} else {
		return errors.New("please give one of (id|serach|lookup) as flag")
	}

	return nil
}

func adminDelfollow(c *cli.Context) error {
	if c.NArg() < 2 {
		return errors.New("please give skipchain-id and ip:port to delete")
	}
	cfg := getConfigOrFail(c)
	scid, err := hex.DecodeString(c.Args().First())
	if err != nil {
		return err
	}
	link, err := findLinkFromAddress(cfg, c.Args().Get(1))
	if err != nil {
		return err
	}
	cerr := skipchain.NewClient().DelFollow(link.Conode, link.Private, scid)
	if cerr != nil {
		return cerr
	}
	log.Infof("Successfully deleted following %x on %s", scid, link.Conode)
	return nil
}

func adminList(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("please give ip:port of the host to list")
	}
	cfg := getConfigOrFail(c)
	link, err := findLinkFromAddress(cfg, c.Args().First())
	if err != nil {
		return err
	}
	list, cerr := skipchain.NewClient().ListFollow(link.Conode, link.Private)
	if cerr != nil {
		return cerr
	}
	if list.FollowIDs != nil {
		log.Info("Followed skipchains:")
		for _, id := range *list.FollowIDs {
			log.Infof("%x", id)
		}
	}
	if list.Follow != nil {
		log.Info("Skipchains where new blocks might be accepted:")
		for _, fct := range *list.Follow {
			follow := []string{"None", "String", "AnyNode"}[fct.NewChain]
			log.Infof("Following '%s' for: %x", follow, fct.Block.SkipChainID())
		}
	}
	return nil
}

// Creates a new skipchain with the given roster
func scCreate(c *cli.Context) error {
	cfg := getConfigOrFail(c)
	log.Info("Create skipchain")
	group := readGroup(c, 0)
	client := skipchain.NewClient()
	var priv kyber.Scalar
	remote, found := cfg.Values.Link[group.Roster.List[0].Public.String()]
	if found {
		priv = remote.Private
	}
	sb, cerr := client.CreateGenesisSignature(group.Roster, c.Int("base"), c.Int("height"),
		skipchain.VerificationStandard, nil, nil, priv)
	if cerr != nil {
		log.Fatal("while creating the genesis-roster:", cerr)
	}
	log.Infof("Created new skipblock with id %x", sb.Hash)
	cfg.Db.Store(sb)
	log.ErrFatal(cfg.save(c))
	return nil
}

// Joins a given skipchain
func lsJoin(c *cli.Context) error {
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
		log.Error(cerr)
		return cerr
	}
	latest := gcr.Update[len(gcr.Update)-1]
	genesis := latest.GenesisID
	if genesis == nil {
		genesis = latest.Hash
	}
	log.Infof("Joined skipchain %x", genesis)
	cfg := getConfigOrFail(c)
	cfg.Db.Store(latest)
	log.ErrFatal(cfg.save(c))
	return nil
}

// Proposes a new block to the leader for appending to the skipchain.
func scAdd(c *cli.Context) error {
	log.Info("Adding a block with a new group")
	if c.NArg() < 2 {
		return errors.New("Please give group-file and id to add")
	}
	group := readGroup(c, 1)
	cfg := getConfigOrFail(c)
	sb := cfg.Db.GetFuzzy(c.Args().First())
	if sb == nil {
		return errors.New("didn't find latest block - update first")
	}
	client := skipchain.NewClient()
	guc, cerr := client.GetUpdateChain(sb.Roster, sb.Hash)
	if cerr != nil {
		return cerr
	}
	latest := guc.Update[len(guc.Update)-1]
	var priv kyber.Scalar
	link := cfg.Values.Link[group.Roster.List[0].Public.String()]
	if link != nil {
		log.Lvl1("Found link-entry for", group.Roster.List[0].Address)
		priv = link.Private
	}
	ssbr, cerr := client.StoreSkipBlockSignature(latest, group.Roster, nil, priv)
	if cerr != nil {
		return errors.New("while storing block: " + cerr.Error())
	}
	cfg.Db.Store(ssbr.Latest)
	log.ErrFatal(cfg.save(c))
	log.Infof("Added new block %x to chain %x", ssbr.Latest.Hash, ssbr.Latest.GenesisID)
	return nil
}

// Updates a block to the latest block
func scUpdate(c *cli.Context) error {
	log.Info("Updating block")
	if c.NArg() < 1 {
		return errors.New("please give block-id to update")
	}
	cfg := getConfigOrFail(c)

	sb := cfg.Db.GetFuzzy(c.Args().First())
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
			cfg.Db.Store(b)
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
	if cfg.Db.Length() == 0 {
		log.Info("Didn't find any blocks yet")
		return nil
	}
	for _, g := range cfg.getSortedGenesis() {
		short := !c.Bool("long")
		log.Info(g.Sprint(short))
		sub := sbli{}
		sbs, err := cfg.Db.GetSkipchains()
		if err != nil {
			return err
		}
		for _, sb := range sbs {
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
	sbs, err := cfg.Db.GetSkipchains()
	if err != nil {
		return err
	}
	for _, sb := range sbs {
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
			cfg.Db.Store(sb)
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
	group, err := app.ReadGroupDescToml(f, cothority.Suite)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s",
			name)
	}
	return group
}

func getConfigOrFail(c *cli.Context) *config {
	cfg, err := loadConfig(c)
	if err != nil {
		log.Fatal("couldn't read config: " + err.Error())
	}
	return cfg
}

func loadConfig(c *cli.Context) (*config, error) {
	cfgPath := app.TildeToHome(c.GlobalString("config"))
	dir := path.Dir(cfgPath)
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(dir, 0770)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	cfg := &config{
		Values: &values{Link: map[string]*link{}},
	}
	_, err = os.Stat(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			db, err := bolt.Open(cfgPath, 0600, nil)
			if err != nil {
				return nil, err
			}
			db.Update(func(tx *bolt.Tx) error {
				_, err := tx.CreateBucket([]byte("skipblocks"))
				if err != nil {
					return fmt.Errorf("create bucket: %s", err)
				}
				_, err = tx.CreateBucket([]byte("config"))
				if err != nil {
					return fmt.Errorf("create bucket: %s", err)
				}
				return nil
			})
			cfg.Db = skipchain.NewSkipBlockDB(db, "skipblocks")
			return cfg, nil
		}
		return nil, fmt.Errorf("Could not open file %s", cfgPath)
	}
	db, err := bolt.Open(cfgPath, 0600, nil)
	if err != nil {
		return nil, err
	}
	cfg.Db = skipchain.NewSkipBlockDB(db, "skipblocks")
	err = cfg.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		v := b.Get([]byte("values"))
		if v != nil {
			_, val, err := network.Unmarshal(v, cothority.Suite)
			if err != nil {
				return err
			}
			vals, ok := val.(*values)
			if !ok {
				return errors.New("stored bytes are not 'values'")
			}
			if len(vals.Link) > 0 {
				cfg.Values.Link = vals.Link
			}
		}
		return nil
	})
	return cfg, err
}

func (cfg *config) save(c *cli.Context) error {
	buf, err := network.Marshal(cfg.Values)
	if err != nil {
		return err
	}
	err = cfg.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		err := b.Put([]byte("values"), buf)
		return err
	})
	if err != nil {
		return err
	}
	return cfg.Db.Close()
}

func (cfg *config) getSortedGenesis() []*skipchain.SkipBlock {
	genesis := sbl{}
	sbs, err := cfg.Db.GetSkipchains()
	if err != nil {
		log.Error(err)
		return nil
	}
	for _, sb := range sbs {
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

func findLinkFromAddress(cfg *config, address string) (*link, error) {
	var l *link
	// Else search for the ip:port
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("invalid host:port option: " + err.Error())
	}
	resolved, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return nil, errors.New("invalid host: " + err.Error())
	}
	ipPort := net.JoinHostPort(resolved.String(), port)
	for _, o := range cfg.Values.Link {
		if o.Address == network.NewTCPAddress(ipPort) {
			l = o
			break
		}
	}
	if l == nil {
		return nil, errors.New("no such link found")
	}
	return l, nil
}
