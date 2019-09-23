// The skipchain-manager (scmgr) is a CLI which lets you create, modify and
// query skipchains.
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.etcd.io/bbolt"
)

var bucketName = []byte("skipblocks")

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

func init() {
	network.RegisterMessages(&config{}, &values{})
}

var gitTag = "dev"

func main() {
	rand.Seed(time.Now().Unix())

	cliApp := cli.NewApp()
	cliApp.Name = "scmgr"
	cliApp.Usage = "Create, modify and query skipchains"
	cliApp.Version = gitTag
	cliApp.Commands = getCommands()
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name: "config, c",
			// we use GetDataPath because only non-human-readable
			// data files are stored here
			Value: cfgpath.GetDataPath("scmgr"),
			Usage: "path to config-file",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	log.ErrFatal(cliApp.Run(os.Args))
}

// linkAdd tries to store our public key in the conode
func linkAdd(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("please give private.toml file")
	}
	private := c.Args().First()
	if private == "" {
		return errors.New("got empty private.toml file")
	}
	ccfg, err := app.LoadCothority(c.Args().First())
	if err != nil {
		return err
	}
	si, err := ccfg.GetServerIdentity()
	if err != nil {
		return err
	}

	cfg := getConfigOrFail(c)
	kp := key.NewKeyPair(cothority.Suite)
	cfg.Values.Link[si.Public.String()] = &link{
		Private: kp.Private,
		Address: si.Address,
		Conode:  si,
	}
	log.Infof("Connecting to %s and creating link", si.Address)
	err = skipchain.NewClient().CreateLinkPrivate(si, si.GetPrivate(), kp.Public)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Info("Correctly linked with", si.Address)
	return cfg.save(c)
}

func linkDel(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("please give IP:Port of the conode to unlink")
	}
	cfg := getConfigOrFail(c)
	link, err := findLinkFromAddress(cfg, c.Args().First())
	if err != nil {
		return err
	}
	err = skipchain.NewClient().Unlink(link.Conode, link.Private)
	if err != nil {
		return errors.New("couldn't unlink:" + err.Error())
	}
	log.Info("Successfully unlinked with", link.Conode)
	return nil
}
func linkList(c *cli.Context) error {
	cfg := getConfigOrFail(c)
	for _, link := range cfg.Values.Link {
		log.Infof("Linked public key for conode %s: %s", link.Address,
			cothority.Suite.Point().Mul(link.Private, nil))
	}
	return nil
}
func linkQuery(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("please give ip:port of conode to query")
	}
	si := network.NewServerIdentity(nil, network.NewAddress(network.PlainTCP, c.Args().First()))
	log.Infof("Contacting node %s to list client public keys",
		si.Address)
	keys, err := skipchain.NewClient().Listlink(si)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		log.Infof("Node %s does not have any public keys stored", si.Address)
	} else {
		log.Infof("Node %s is secured and has the following public keys stored:", si.Address)
		for _, pub := range keys {
			log.Infof("Public-key: %s", pub)
		}
	}
	return nil
}

func followAddID(c *cli.Context) error {
	cfg := getConfigOrFail(c)
	if c.NArg() != 2 {
		return errors.New("please give: skipchain_id ip:port")
	}
	scid, err := hex.DecodeString(c.Args().First())
	if err != nil {
		return errors.New("invalid skipchain-id: " + err.Error())
	}
	link, err := findLinkFromAddress(cfg, c.Args().Get(1))
	if err != nil {
		return errors.New("couldn't parse node-address or not linked yet: " + err.Error())
	}
	log.Infof("Adding skipchain %x to conode %s", scid, link.Address)
	err = skipchain.NewClient().AddFollow(link.Conode, link.Private, scid, skipchain.FollowID,
		skipchain.NewChainNone, "")
	if err != nil {
		return errors.New("couldn't add this block as chain-follower: " + err.Error())
	}
	return nil
}
func followAddRoster(c *cli.Context) error {
	cfg := getConfigOrFail(c)
	if c.NArg() != 2 {
		return errors.New("please give the following: [--lookup ip:port] [--any] ID ip:port")
	}
	scid, err := hex.DecodeString(c.Args().First())
	if err != nil {
		return errors.New("invalid skipchain-id: " + err.Error())
	}
	link, err := findLinkFromAddress(cfg, c.Args().Get(1))
	if err != nil {
		return errors.New("couldn't parse node-address or not linked yet: " + err.Error())
	}
	scURL := c.String("lookup")

	log.Infof("Allowing conode %s to be used as node in roster of skipchain %x", link.Conode.Address,
		scid)
	nc := skipchain.NewChainStrictNodes
	if c.Bool("any") {
		log.Info("Allowing use of conode if _any_ node matches the roster.")
		nc = skipchain.NewChainAnyNode
	}
	ft := skipchain.FollowSearch
	if scURL != "" {
		log.Infof("Will try to lookup the skipchain on conode %s", scURL)
		ft = skipchain.FollowLookup
	}
	err = skipchain.NewClient().AddFollow(link.Conode, link.Private, scid, ft,
		nc, scURL)
	if err != nil {
		return errors.New("couldn't find this block in search: " + err.Error())
	}
	return nil
}
func followDel(c *cli.Context) error {
	if c.NArg() != 2 {
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
	log.Infof("Deleting following of skipchain %x in conode %s", scid, link.Conode.Address)
	err = skipchain.NewClient().DelFollow(link.Conode, link.Private, scid)
	if err != nil {
		return err
	}
	log.Infof("Successfully deleted following of skipchain %x in conode %v.", scid, link.Conode)
	return nil
}
func followList(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("please give ip:port of the host to list")
	}
	cfg := getConfigOrFail(c)
	link, err := findLinkFromAddress(cfg, c.Args().First())
	if err != nil {
		return err
	}
	list, err := skipchain.NewClient().ListFollow(link.Conode, link.Private)
	if err != nil {
		return err
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
	if list.FollowIDs != nil && list.Follow != nil {
		log.Info("Conode doesn't follow any skipchain and allows everything.")
	}
	return nil
}

// Creates a new skipchain with the given roster
func scCreate(c *cli.Context) error {
	cfg := getConfigOrFail(c)
	group := readGroupArgs(c, 0)
	var priv kyber.Scalar
	remote, found := cfg.Values.Link[group.Roster.List[0].Public.String()]
	if found {
		log.Infof("Found link for %s and using signed request", remote.Address)
		priv = remote.Private
	} else {
		log.Infof("Trying to connect without signature to %s", group.Roster.List[0].Address)
	}
	log.Infof("Creating new skipchain with leader %s and roster %s.", group.Roster.List[0], group.Roster)
	log.Infof("Base-height: %d; Maximum-height: %d", c.Int("base"), c.Int("height"))
	sb, err := skipchain.NewClient().CreateGenesisSignature(group.Roster, c.Int("base"), c.Int("height"),
		skipchain.VerificationStandard, nil, priv)
	if err != nil {
		return errors.New("while creating the genesis-roster: " + err.Error())
	}
	log.Infof("Created new skipblock with id %x", sb.Hash)
	cfg.Db.Store(sb)
	log.ErrFatal(cfg.save(c))
	return nil
}

// Gets blocks from -from ID to the end of the chain.
func scUpdates(c *cli.Context) error {
	group := readGroupArgs(c, 0)

	if c.String("from") == "" {
		return errors.New("-from argument required")
	}

	from, err := hex.DecodeString(c.String("from"))
	if err != nil {
		return errors.New("Failed to decode -from " + c.String("from"))
	}
	if len(from) == 0 {
		return errors.New("-from argument is empty")
	}

	r, err := skipchain.NewClient().GetUpdateChainLevel(group.Roster, from, c.Int("level"), c.Int("count"))
	if err != nil {
		return err
	}

	for _, x := range r {
		fmt.Fprintf(c.App.Writer, "index %v, hash %x\n", x.Index, x.Hash)
		for li, fl := range x.ForwardLink {
			if fl.NewRoster != nil {
				fmt.Fprintf(c.App.Writer, "  forward link %v, newRoster %v\n", li, fmtRoster(fl.NewRoster))
			} else {
				fmt.Fprintf(c.App.Writer, "  forward link %v\n", li)
			}
		}
	}

	return nil
}

func fmtRoster(r *onet.Roster) string {
	var roster []string
	for _, s := range r.List {
		if s.URL != "" {
			roster = append(roster, fmt.Sprintf("%v (url: %v)", string(s.Address), s.URL))
		} else {
			roster = append(roster, string(s.Address))
		}
	}
	return strings.Join(roster, ", ")
}

// Proposes a new block to the leader for appending to the skipchain.
func scAdd(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("Please give a skipchain-id")
	}

	cfg := getConfigOrFail(c)
	sb, err := cfg.Db.GetFuzzy(c.Args().First())
	if err != nil {
		return err
	}
	if sb == nil {
		return errors.New("didn't find this skipchain")
	}
	log.Info("Updating the skipchain to know where to add a new block.")
	guc, err := skipchain.NewClient().GetUpdateChain(sb.Roster, sb.Hash)
	if err != nil {
		return err
	}
	if len(guc.Update) == 0 {
		return errors.New("no latest block")
	}
	latest := guc.Update[len(guc.Update)-1]

	var roster *onet.Roster
	if rosterFile := c.String("roster"); rosterFile != "" {
		group := readGroup(rosterFile)
		if group == nil {
			return errors.New("Error while reading group definition file: " + rosterFile)
		}
		if len(group.Roster.List) == 0 {
			return errors.New("Empty entity or invalid group defintion in: " +
				rosterFile)
		}
		roster = group.Roster
	} else {
		roster = sb.Roster
	}

	data := c.String("data")
	dataMsg := []byte(data)

	var priv kyber.Scalar
	link := cfg.Values.Link[roster.List[0].Public.String()]
	if link != nil {
		log.Info("Found link-entry for", roster.List[0].Address)
		priv = link.Private
	}

	log.Info("Adding new block to skipchain.")
	ssbr, err := skipchain.NewClient().StoreSkipBlockSignature(latest, roster, dataMsg, priv)
	if err != nil {
		return errors.New("while storing block: " + err.Error())
	}
	cfg.Db.Store(ssbr.Latest)
	log.ErrFatal(cfg.save(c))
	log.Infof("Added new block %x to chain %x", ssbr.Latest.Hash, ssbr.Latest.SkipChainID())
	return nil
}

// Prints details about a SkipBlock
func scPrint(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("Please give a skipchain-id")
	}
	cfg := getConfigOrFail(c)
	sb, err := cfg.Db.GetFuzzy(c.Args().First())
	if err != nil {
		return err
	}
	if sb == nil {
		return errors.New("didn't find this skipblock")
	}
	log.Info("Content of skipblock:")
	log.Infof("Index: %d", sb.Index)
	for i, bl := range sb.BackLinkIDs {
		log.Infof("BackwardLink[%d] = %x", i, bl)
	}
	for i, fl := range sb.ForwardLink {
		log.Infof("ForwardLink[%d] = %x", i, fl.To)
	}
	log.Infof("Data: %#v", string(sb.Data))
	for i, vf := range sb.VerifierIDs {
		vfStr := vf.String()
		switch vf {
		case skipchain.VerifyBase:
			vfStr = "skipchain.VerifyBase"
		case byzcoin.Verify:
			vfStr = "byzcoin.Verify"
		}
		log.Infof("Verification[%d] = %s", i, vfStr)
	}
	log.Infof("SkipchainID: %x", sb.SkipChainID())
	log.Infof("Hash/SkipblockID: %x", sb.Hash)
	return nil
}

func scOptimize(c *cli.Context) error {
	rosterFile := c.String("roster")
	if rosterFile == "" {
		return errors.New("Missing roster file")
	}

	group := readGroup(rosterFile)
	if group == nil {
		return errors.New("Error while reading group definition file: " + rosterFile)
	}
	roster := group.Roster

	blockID := c.String("id")
	if blockID == "" {
		return errors.New("Missing block ID")
	}

	id, err := hex.DecodeString(blockID)
	if err != nil {
		return errors.New("bad block ID")
	}

	cl := skipchain.NewClient()

	sb, err := cl.GetSingleBlock(roster, id)
	if err != nil {
		return fmt.Errorf("couldn't get the block: %v", err)
	}

	// If a genesis block is provided, we optimize the entire chain
	if sb.Index == 0 {
		reply, err := cl.GetUpdateChain(roster, id)
		if err != nil {
			return fmt.Errorf("couldn't get the latest block: %v", err)
		}

		sb = reply.Update[len(reply.Update)-1]
	}

	log.Infof("Optimizing chain %x for block at index %d...", sb.SkipChainID(), sb.Index)

	reply, err := cl.OptimizeProof(roster, sb.Hash)
	if err != nil {
		return fmt.Errorf("couldn't optimize the proof: %v", err)
	}

	log.Infof("Chain optimized with %d blocks", len(reply.Proof))
	return nil
}

// Joins a given skipchain
func dnsFetch(c *cli.Context) error {
	if c.NArg() != 2 {
		return errors.New("Please give group-file and id of skipchain")
	}
	group := readGroupArgs(c, 0)
	sbid, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return err
	}
	log.Infof("Requesting latest block attached to %x", sbid)
	gcr, err := skipchain.NewClient().GetUpdateChain(group.Roster, sbid)
	if err != nil {
		log.Error(err)
		return err
	}
	latest := gcr.Update[len(gcr.Update)-1]
	genesis := latest.SkipChainID()
	cfg := getConfigOrFail(c)
	cfg.Db.Store(latest)
	if cfg.Db.GetByID(genesis) == nil {
		genesisBlock, err := skipchain.NewClient().GetSingleBlock(group.Roster, genesis)
		if err != nil {
			return err
		}
		cfg.Db.Store(genesisBlock)
	}
	log.Infof("Fetched skipchain with id: %x", genesis)
	log.ErrFatal(cfg.save(c))
	return nil
}

// lsKnown shows all known skipblocks
func dnsList(c *cli.Context) error {
	cfg, err := loadConfig(c)
	if err != nil {
		return errors.New("couldn't read config: " + err.Error())
	}
	if cfg.Db.Length() == 0 {
		log.Info("Didn't find any blocks yet")
		return nil
	}
	log.Info("List of all stored skipchains:")
	for _, g := range cfg.getSortedGenesis() {
		short := !c.Bool("long")
		log.Info(g.Sprint(short))
		sub := sbli{}
		sbs, err := cfg.Db.GetSkipchains()
		if err != nil {
			return err
		}
		for _, sb := range sbs {
			if sb.SkipChainID().Equal(g.Hash) && sb.Index > 0 {
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

// lsIndex writes one index-file for every known skipchain and an index.js
// for all skiplchains.
func dnsIndex(c *cli.Context) error {
	output := c.Args().First()
	if len(output) == 0 {
		return errors.New("Missing output path")
	}

	cleanJSFiles(output)

	cfg, err := loadConfig(c)
	if err != nil {
		return errors.New("couldn't read config: " + err.Error())
	}

	// Get the list of genesis block
	genesis := cfg.getSortedGenesis()

	// Build the json structure
	blocks := jsonBlockList{}
	blocks.Blocks = make([]jsonBlock, len(genesis))
	log.Info("Going through all skipchain-ids and writing the genesis-block")
	for i, g := range genesis {
		block := &blocks.Blocks[i]
		block.SkipchainID = hex.EncodeToString(g.Hash)
		block.Servers = make([]string, len(g.Roster.List))
		block.Data = g.Data

		for j, server := range g.Roster.List {
			block.Servers[j] = net.JoinHostPort(server.Address.Host(), server.Address.Port())
		}

		// Write the genesis block file
		content, _ := json.Marshal(block)
		log.Infof("Writing %s.js", block.SkipchainID)
		err := ioutil.WriteFile(filepath.Join(output, block.SkipchainID+".js"), content, 0644)

		if err != nil {
			log.Info("Cannot write block-specific file")
		}
	}

	content, err := json.Marshal(blocks)
	if err != nil {
		log.Info("Cannot convert to json")
	}

	// Write the json into the index.js
	log.Infof("Storing an index of all blocks to index.js")
	err = ioutil.WriteFile(filepath.Join(output, "index.js"), content, 0644)
	if err != nil {
		log.Info("Cannot write in the file")
	}

	return nil
}

func dnsUpdate(c *cli.Context) error {
	cfg := getConfigOrFail(c)
	fetchNew := c.Bool("new")
	sisAll := map[network.ServerIdentityID]*network.ServerIdentity{}
	var roster *onet.Roster
	if groupFile := c.String("roster"); groupFile != "" {
		group := readGroup(groupFile)
		if group == nil {
			return errors.New("invalid group-file: " + groupFile)
		}
		roster = group.Roster
	}
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
	sisNew = updateNewSIs(roster, sisNew, sisAll)

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
		gasr, err := client.GetAllSkipchains(si)
		if err != nil {
			// Error is not fatal here - perhaps the node is down,
			// but we can continue anyway.
			log.Error(err)
			continue
		}
		for _, sb := range gasr.SkipChains {
			if cfg.Db.GetByID(sb.SkipChainID()) == nil {
				if !fetchNew {
					log.Lvlf2("Ignoring unknown skipchain %x", sb.SkipChainID())
					continue
				}
				log.Lvl1("Adding new roster to search")
				sisNew = updateNewSIs(sb.Roster, sisNew, sisAll)
			}
			log.Infof("Found skipchain %x", sb.SkipChainID())
			cfg.Db.Store(sb)
		}
	}
	return cfg.save(c)
}

// Remove every file matching *.js in the given directory
func cleanJSFiles(dir string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".js") {
			err := os.Remove(filepath.Join(dir, f.Name()))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// JSON skipblock element to be written in the index.js file
type jsonBlock struct {
	SkipchainID string
	Servers     []string
	Data        []byte
}

// JSON list of skipblocks element to be written in the index.js file
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

func readGroupArgs(c *cli.Context, pos int) *app.Group {
	if c.NArg() <= pos {
		log.Fatal("Please give the group-file as argument")
	}
	name := c.Args().Get(pos)
	return readGroup(name)
}
func readGroup(name string) *app.Group {
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
	if err != nil {
		log.Fatal("couldn't read config: " + err.Error())
	}
	return cfg
}

func loadConfig(c *cli.Context) (*config, error) {
	cfgPath := path.Join(c.GlobalString("config"), "config.bin")
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
			db, err := bbolt.Open(cfgPath, 0600, nil)
			if err != nil {
				return nil, err
			}
			db.Update(func(tx *bbolt.Tx) error {
				_, err := tx.CreateBucket(bucketName)
				if err != nil {
					return fmt.Errorf("create bucket: %s", err)
				}
				_, err = tx.CreateBucket([]byte("config"))
				if err != nil {
					return fmt.Errorf("create bucket: %s", err)
				}
				return nil
			})
			cfg.Db = skipchain.NewSkipBlockDB(db, bucketName)
			return cfg, nil
		}
		return nil, fmt.Errorf("Could not open file %s", cfgPath)
	}
	db, err := bbolt.Open(cfgPath, 0600, nil)
	if err != nil {
		return nil, err
	}
	cfg.Db = skipchain.NewSkipBlockDB(db, bucketName)
	err = cfg.Db.View(func(tx *bbolt.Tx) error {
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
	err = cfg.Db.Update(func(tx *bbolt.Tx) error {
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
	if roster == nil {
		return sisNew
	}
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
	for _, o := range cfg.Values.Link {
		if o.Address.NetworkAddress() == address {
			l = o
			break
		}
	}
	if l == nil {
		return nil, errors.New("no such link found")
	}
	return l, nil
}
