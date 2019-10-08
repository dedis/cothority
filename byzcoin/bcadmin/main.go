package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/xerrors"

	"github.com/qantik/qrgo"
	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	_ "go.dedis.ch/cothority/v3/eventlog"
	_ "go.dedis.ch/cothority/v3/personhood"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

type chainFetcher func(si *network.ServerIdentity) ([]skipchain.SkipBlockID, error)

const errUnregisteredMessage = "The requested message hasn't been registered"

func init() {
	network.RegisterMessages(&darc.Darc{}, &darc.Identity{}, &darc.Signer{})
}

var cliApp = cli.NewApp()

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

var gitTag = "dev"

func init() {
	cliApp.Name = lib.BcaName
	cliApp.Usage = "Create ByzCoin ledgers and grant access to them."
	cliApp.Version = gitTag
	cliApp.Commands = cmds // stored in "commands.go"
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:   "config, c",
			EnvVar: "BC_CONFIG",
			Value:  getDataPath(lib.BcaName),
			Usage:  "path to configuration-directory",
		},
		cli.BoolFlag{
			Name:   "wait, w",
			EnvVar: "BC_WAIT",
			Usage:  "wait for transaction available in all nodes",
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
		log.Fatalf("error: %+v", err)
	}
	return
}

func create(c *cli.Context) error {
	fn := c.String("roster")
	if fn == "" {
		fn = c.Args().First()
		if fn == "" {
			return errors.New("roster argument or --roster flag is required")
		}
	}
	r, err := lib.ReadRoster(fn)
	if err != nil {
		return err
	}

	interval := c.Duration("interval")

	owner := darc.NewSignerEd25519(nil, nil)

	req, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, r, []string{"spawn:longTermSecret"}, owner.Identity())
	if err != nil {
		log.Error(err)
		return err
	}
	req.BlockInterval = interval

	cl, resp, err := byzcoin.NewLedger(req, false)
	if err != nil {
		return err
	}

	cfg := lib.Config{
		ByzCoinID:     resp.Skipblock.SkipChainID(),
		Roster:        *r,
		AdminDarc:     req.GenesisDarc,
		AdminIdentity: owner.Identity(),
	}
	fn, err = lib.SaveConfig(cfg)
	if err != nil {
		return err
	}

	err = lib.SaveKey(owner)
	if err != nil {
		return err
	}

	log.Infof("Created ByzCoin with ID %x.\n", cfg.ByzCoinID)
	fmt.Printf("\nexport BC=\"%s\"\n", fn)

	// For the tests to use.
	c.App.Metadata["BC"] = fn

	return lib.WaitPropagation(c, cl)
}

func link(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("please give the following args: roster.toml [byzcoin id]")
	}
	r, err := lib.ReadRoster(c.Args().First())
	if err != nil {
		return err
	}

	if c.NArg() == 1 {
		log.Info("Fetching all byzcoin-ids from the roster")
		var scIDs []skipchain.SkipBlockID
		for _, si := range r.List {
			ids, err := fetchChains(si, byzcoinFetcher, skipchainFetcher)
			if err != nil {
				log.Warn("Couldn't contact", si.Address, err)
			} else {
				scIDs = append(scIDs, ids...)
				log.Infof("Got %d id(s) from %s", len(ids), si.Address)
			}
		}
		sort.Slice(scIDs, func(i, j int) bool {
			return bytes.Compare(scIDs[i], scIDs[j]) < 0
		})
		for i := len(scIDs) - 1; i > 0; i-- {
			if scIDs[i].Equal(scIDs[i-1]) {
				scIDs = append(scIDs[0:i], scIDs[i+1:]...)
			}
		}
		log.Info("All IDs available in this roster:")
		for _, id := range scIDs {
			log.Infof("%x", id[:])
		}
	} else {
		id, err := hex.DecodeString(c.Args().Get(1))
		if err != nil || len(id) != 32 {
			return errors.New("second argument is not a valid ID")
		}
		var cl *byzcoin.Client
		var cc *byzcoin.ChainConfig
		for _, si := range r.List {
			ids, err := fetchChains(si, byzcoinFetcher, skipchainFetcher)
			if err != nil {
				log.Warn("Got error while asking", si.Address, "for skipchains:", err)
			}
			found := false
			for _, idc := range ids {
				if idc.Equal(id) {
					found = true
					break
				}
			}
			if found {
				cl = byzcoin.NewClient(id, *onet.NewRoster([]*network.ServerIdentity{si}))
				cc, err = cl.GetChainConfig()
				if err != nil {
					cl = nil
					log.Warnf("Could not get chain config from %v: %+v\n", si, err)
					continue
				}
				cl.Roster = cc.Roster
				break
			}
		}
		if cl == nil {
			return errors.New("didn't manage to find a node with a valid copy of the given skipchain-id")
		}

		newDarc := &darc.Darc{}

		dstr := c.String("darc")
		if dstr == "" {
			log.Info("no darc given, will use an empty default one")
		} else {

			// Accept both plain-darcs, as well as "darc:...." darcs
			darcID, err := lib.StringToDarcID(dstr)
			if err != nil {
				return errors.New("failed to parse darc: " + err.Error())
			}

			p, err := cl.GetProofFromLatest(darcID)
			if err != nil {
				return errors.New("couldn't get proof for darc: " + err.Error())
			}

			_, darcBuf, cid, _, err := p.Proof.KeyValue()
			if err != nil {
				return errors.New("cannot get value for darc: " + err.Error())
			}

			if cid != byzcoin.ContractDarcID {
				return errors.New("please give a darc-instance ID, not: " + cid)
			}

			newDarc, err = darc.NewFromProtobuf(darcBuf)
			if err != nil {
				return errors.New("invalid darc stored in byzcoin: " + err.Error())
			}
		}

		identity := cothority.Suite.Point()

		identityStr := c.String("identity")
		if identityStr == "" {
			log.Info("no identity provided, will use a default one")
		} else {
			identityBuf, err := lib.StringToEd25519Buf(identityStr)
			if err != nil {
				return err
			}

			identity = cothority.Suite.Point()
			err = identity.UnmarshalBinary(identityBuf)
			if err != nil {
				return errors.New("got an invalid identity: " + err.Error())
			}
		}

		log.Infof("ByzCoin-config for %+x:\n"+
			"\tRoster: %s\n"+
			"\tBlockInterval: %s\n"+
			"\tMacBlockSize: %d\n"+
			"\tDarcContracts: %s",
			id[:], cc.Roster.List, cc.BlockInterval, cc.MaxBlockSize, cc.DarcContractIDs)
		filePath, err := lib.SaveConfig(lib.Config{
			Roster:        cc.Roster,
			ByzCoinID:     id,
			AdminDarc:     *newDarc,
			AdminIdentity: darc.NewIdentityEd25519(identity),
		})
		if err != nil {
			return errors.New("while writing config-file: " + err.Error())
		}
		log.Info(fmt.Sprintf("Wrote config to \"%s\"", filePath))
	}
	return nil
}

func byzcoinFetcher(si *network.ServerIdentity) ([]skipchain.SkipBlockID, error) {
	cl := byzcoin.NewClient(nil, onet.Roster{})
	reply, err := cl.GetAllByzCoinIDs(si)
	if err != nil {
		return nil, err
	}

	return reply.IDs, nil
}

func skipchainFetcher(si *network.ServerIdentity) ([]skipchain.SkipBlockID, error) {
	cl := skipchain.NewClient()
	reply, err := cl.GetAllSkipChainIDs(si)
	if err != nil {
		return nil, err
	}

	return reply.IDs, nil
}

func fetchChains(si *network.ServerIdentity, fns ...chainFetcher) ([]skipchain.SkipBlockID, error) {
	for _, fn := range fns {
		ids, err := fn(si)
		if err != nil {
			if !strings.Contains(err.Error(), errUnregisteredMessage) {
				return nil, err
			}
		} else {
			return ids, nil
		}
	}

	return nil, errors.New("couldn't find registered handler")
}

func latest(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		bcArg = c.Args().First()
		if bcArg == "" {
			return errors.New("--bc flag is required")
		}
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	// Allow the user to set the server number; useful when testing leader rotation.
	sn := c.Int("server")
	if sn >= 0 {
		err := cl.UseNode(sn)
		if err != nil {
			return err
		}
	}

	log.Infof("ByzCoinID: %x\n", cfg.ByzCoinID)
	log.Infof("Admin DARC: %x\n", cfg.AdminDarc.GetBaseID())
	_, err = fmt.Fprintln(c.App.Writer, "local roster:", fmtRoster(&cfg.Roster))
	if err != nil {
		return err
	}
	if sn >= 0 {
		_, err = fmt.Fprintln(c.App.Writer, "contacting server:", cl.Roster.List[sn])
		if err != nil {
			return err
		}
	}

	// Find the latest block by asking for the Proof of the config instance.
	p, err := cl.GetProofFromLatest(byzcoin.ConfigInstanceID.Slice())
	if err != nil {
		return err
	}

	sb := p.Proof.Latest
	log.Infof("Last block:\n\tIndex: %d\n\tBlockMaxHeight: %d\n\tBackLinks: %d\n\tRoster: %s\n\n",
		sb.Index, sb.Height, len(sb.BackLinkIDs), fmtRoster(sb.Roster))

	if c.Bool("roster") {
		g := &app.Group{Roster: sb.Roster}
		gt, err := g.Toml(cothority.Suite)
		if err != nil {
			return err
		}
		fmt.Fprintln(c.App.Writer, gt.String())
	}

	if c.Bool("update") {
		cfg.Roster = *sb.Roster
		var fn string
		fn, err = lib.SaveConfig(cfg)
		if err == nil {
			_, err = fmt.Fprintln(c.App.Writer, "updated config file:", fn)
			if err != nil {
				return err
			}
		}
	}

	if c.Bool("header") {
		var header byzcoin.DataHeader
		if err = protobuf.Decode(sb.Data, &header); err != nil {
			return err
		}

		fmt.Fprintf(c.App.Writer, "Header:\n\tTrieRoot: %x\n\tTimestamp: %d\n\tVersion: %d\n", header.TrieRoot, header.Timestamp, header.Version)
	}

	return err
}

func fmtRoster(r *onet.Roster) string {
	var roster []string
	for _, s := range r.List {
		if s.URL != "" {
			roster = append(roster, fmt.Sprintf("%v (url: %+v)", string(s.Address), s.URL))
		} else {
			roster = append(roster, string(s.Address))
		}
	}
	return strings.Join(roster, ", ")
}

func getBcKey(c *cli.Context) (cfg lib.Config, cl *byzcoin.Client, signer *darc.Signer,
	proof byzcoin.Proof, chainCfg byzcoin.ChainConfig, err error) {
	if c.NArg() < 2 {
		err = errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg")
		return
	}
	cfg, cl, err = lib.LoadConfig(c.Args().First())
	if err != nil {
		err = errors.New("couldn't load config file: " + err.Error())
		return
	}
	signer, err = lib.LoadSigner(c.Args().Get(1))
	if err != nil {
		err = errors.New("couldn't load key-xxx.cfg: " + err.Error())
		return
	}

	log.Lvl2("Getting latest chainConfig")
	pr, err := cl.GetProofFromLatest(byzcoin.ConfigInstanceID.Slice())
	if err != nil {
		err = errors.New("couldn't get proof for chainConfig: " + err.Error())
		return
	}
	proof = pr.Proof

	_, value, _, _, err := proof.KeyValue()
	if err != nil {
		err = errors.New("couldn't get value out of proof: " + err.Error())
		return
	}
	err = protobuf.DecodeWithConstructors(value, &chainCfg, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		err = errors.New("couldn't decode chainConfig: " + err.Error())
		return
	}
	cl.Roster = chainCfg.Roster

	return
}

func getBcKeyPub(c *cli.Context) (cfg lib.Config, cl *byzcoin.Client, signer *darc.Signer,
	proof byzcoin.Proof, chainCfg byzcoin.ChainConfig, pub *network.ServerIdentity, err error) {
	cfg, cl, signer, proof, chainCfg, err = getBcKey(c)
	if err != nil {
		return
	}

	fn := c.Args().Get(2)
	if fn == "" {
		err = errors.New("no TOML file provided")
		return
	}
	f, err := os.Open(fn)
	if err != nil {
		return
	}
	defer f.Close()
	group, err := app.ReadGroupDescToml(f)
	if err != nil {
		err = fmt.Errorf("couldn't open %v: %+v", fn, err.Error())
		return
	}
	if len(group.Roster.List) != 1 {
		err = errors.New("the TOML file should have exactly one entry")
		return
	}
	pub = group.Roster.List[0]

	return
}

func updateConfig(cl *byzcoin.Client, signer *darc.Signer, chainConfig byzcoin.ChainConfig) error {
	counters, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return errors.New("couldn't get counters: " + err.Error())
	}
	counters.Counters[0]++
	ccBuf, err := protobuf.Encode(&chainConfig)
	if err != nil {
		return errors.New("couldn't encode chainConfig: " + err.Error())
	}
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.ConfigInstanceID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractConfigID,
			Command:    "update_config",
			Args:       byzcoin.Arguments{{Name: "config", Value: ccBuf}},
		},
		SignerCounter: counters.Counters,
	})
	if err != nil {
		return err
	}

	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return errors.New("couldn't sign the clientTransaction: " + err.Error())
	}

	log.Lvl1("Sending new roster to byzcoin")
	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return errors.New("client transaction wasn't accepted: " + err.Error())
	}
	return nil
}

func config(c *cli.Context) error {
	_, cl, signer, _, chainConfig, err := getBcKey(c)
	if err != nil {
		return err
	}

	if interval := c.String("interval"); interval != "" {
		dur, err := time.ParseDuration(interval)
		if err != nil {
			return errors.New("couldn't parse interval: " + err.Error())
		}
		chainConfig.BlockInterval = dur
	}
	if blockSize := c.Int("blockSize"); blockSize > 0 {
		if blockSize < 16000 && blockSize > 8e6 {
			return errors.New("new blocksize out of bounds: must be between 16e3 and 8e6")
		}
		chainConfig.MaxBlockSize = blockSize
	}

	err = updateConfig(cl, signer, chainConfig)
	if err != nil {
		return err
	}

	log.Lvl1("Updated configuration")

	return lib.WaitPropagation(c, cl)
}

func mint(c *cli.Context) error {
	if c.NArg() < 4 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg pubkey coins")
	}
	cfg, cl, signer, _, _, err := getBcKey(c)
	if err != nil {
		return err
	}

	pubBuf, err := hex.DecodeString(c.Args().Get(2))
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write([]byte(contracts.ContractCoinID))
	h.Write(pubBuf)
	account := byzcoin.NewInstanceID(h.Sum(nil))

	coins, err := strconv.ParseUint(c.Args().Get(3), 10, 64)
	if err != nil {
		return err
	}
	coinsBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(coinsBuf, coins)

	cReply, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return err
	}
	counters := cReply.Counters

	p, err := cl.GetProofFromLatest(account.Slice())
	if err != nil {
		return err
	}
	if !p.Proof.InclusionProof.Match(account.Slice()) {
		log.Info("Creating darc and coin")
		pub := cothority.Suite.Point()
		err = pub.UnmarshalBinary(pubBuf)
		if err != nil {
			return err
		}
		pubI := darc.NewIdentityEd25519(pub)
		rules := darc.NewRules()
		err = rules.AddRule(darc.Action("spawn:coin"), expression.Expr(signer.Identity().String()))
		if err != nil {
			return err
		}
		err = rules.AddRule(darc.Action("invoke:coin.transfer"), expression.Expr(pubI.String()))
		if err != nil {
			return err
		}
		err = rules.AddRule(darc.Action("invoke:coin.mint"), expression.Expr(signer.Identity().String()))
		if err != nil {
			return err
		}
		d := darc.NewDarc(rules, []byte("new coin for mba"))
		dBuf, err := d.ToProto()
		if err != nil {
			return err
		}

		log.Info("Creating darc for coin")
		counters[0]++
		ctx, err := cl.CreateTransaction(byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(cfg.AdminDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: byzcoin.ContractDarcID,
				Args: byzcoin.Arguments{{
					Name:  "darc",
					Value: dBuf,
				}},
			},
			SignerCounter: counters,
		})
		if err != nil {
			return err
		}
		err = ctx.FillSignersAndSignWith(*signer)
		if err != nil {
			return err
		}
		_, err = cl.AddTransactionAndWait(ctx, 10)
		if err != nil {
			return err
		}

		log.Info("Creating coin")
		counters[0]++
		ctx, err = cl.CreateTransaction(byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(d.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: contracts.ContractCoinID,
				Args: byzcoin.Arguments{
					{
						Name:  "type",
						Value: contracts.CoinName.Slice(),
					},
					{
						Name:  "coinID",
						Value: pubBuf,
					},
				},
			},
			SignerCounter: counters,
		})
		if err != nil {
			return err
		}
		err = ctx.FillSignersAndSignWith(*signer)
		if err != nil {
			return err
		}
		_, err = cl.AddTransactionAndWait(ctx, 10)
		if err != nil {
			return err
		}
	}

	log.Info("Minting coin")
	counters[0]++
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: account,
		Invoke: &byzcoin.Invoke{
			ContractID: contracts.ContractCoinID,
			Command:    "mint",
			Args: byzcoin.Arguments{{
				Name:  "coins",
				Value: coinsBuf,
			}},
		},
		SignerCounter: counters,
	})
	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}
	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	log.Infof("Account %x created and filled with %d coins", account[:], coins)

	return lib.WaitPropagation(c, cl)
}

func rosterAdd(c *cli.Context) error {
	if c.NArg() < 3 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg newServer.toml")
	}
	_, cl, signer, _, chainConfig, pub, err := getBcKeyPub(c)
	if err != nil {
		return err
	}

	old := chainConfig.Roster
	if i, _ := old.Search(pub.ID); i >= 0 {
		return errors.New("new node is already in roster")
	}
	log.Lvl2("Old roster is:", old.List)
	chainConfig.Roster = *old.Concat(pub)
	log.Lvl2("New roster is:", chainConfig.Roster.List)

	err = updateConfig(cl, signer, chainConfig)
	if err != nil {
		return err
	}
	log.Lvl1("New roster is now active")

	return lib.WaitPropagation(c, cl)
}

func rosterDel(c *cli.Context) error {
	if c.NArg() < 3 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg serverToDelete.toml")
	}
	_, cl, signer, _, chainConfig, pub, err := getBcKeyPub(c)
	if err != nil {
		return err
	}

	old := chainConfig.Roster
	i, _ := old.Search(pub.ID)
	switch {
	case i < 0:
		return errors.New("node to delete is not in roster")
	case i == 0:
		return errors.New("cannot delete leader from roster")
	}
	log.Lvl2("Old roster is:", old.List)
	list := append(old.List[0:i], old.List[i+1:]...)
	chainConfig.Roster = *onet.NewRoster(list)
	log.Lvl2("New roster is:", chainConfig.Roster.List)

	err = updateConfig(cl, signer, chainConfig)
	if err != nil {
		return err
	}
	log.Lvl1("New roster is now active")

	return lib.WaitPropagation(c, cl)
}

func rosterLeader(c *cli.Context) error {
	if c.NArg() < 3 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg newLeader.toml")
	}
	_, cl, signer, _, chainConfig, pub, err := getBcKeyPub(c)
	if err != nil {
		return err
	}

	old := chainConfig.Roster
	i, _ := old.Search(pub.ID)
	switch {
	case i < 0:
		return errors.New("new leader is not in roster")
	case i == 0:
		return errors.New("new node is already leader")
	}
	log.Lvl2("Old roster is:", old.List)
	list := []*network.ServerIdentity(old.List)
	list[0], list[i] = list[i], list[0]
	chainConfig.Roster = *onet.NewRoster(list)
	log.Lvl2("New roster is:", chainConfig.Roster.List)

	// Do it twice to make sure the new roster is active - there is an issue ;)
	err = updateConfig(cl, signer, chainConfig)
	if err != nil {
		return err
	}
	log.Lvl1("New roster is now active")

	return lib.WaitPropagation(c, cl)
}

func key(c *cli.Context) error {
	if f := c.String("print"); f != "" {
		sig, err := lib.LoadSigner(f)
		if err != nil {
			return errors.New("couldn't load signer: " + err.Error())
		}
		log.Infof("Private: %s\nPublic: %s", sig.Ed25519.Secret, sig.Ed25519.Point)
		return nil
	}
	newSigner := darc.NewSignerEd25519(nil, nil)
	err := lib.SaveKey(newSigner)
	if err != nil {
		return err
	}

	var fo io.Writer

	save := c.String("save")
	if save == "" {
		fo = os.Stdout
	} else {
		file, err := os.Create(save)
		if err != nil {
			return err
		}
		fo = file
		defer func() {
			err := file.Close()
			if err != nil {
				log.Error(err)
			}
		}()
	}
	_, err = fmt.Fprintln(fo, newSigner.Identity().String())
	return err
}

func darcShow(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	dstr := c.String("darc")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}

	d, err := lib.GetDarcByString(cl, dstr)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.App.Writer, d.String())
	return err
}

// "cDesc" stands for Change Description. This function allows one to edit the
// description of a darc.
func darcCdesc(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	desc := c.String("desc")
	if desc == "" {
		return errors.New("--desc flag is required")
	}
	if len(desc) > 1024 {
		return errors.New("descriptions longer than 1024 characters are not allowed")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	dstr := c.String("dstr")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}

	d, err := lib.GetDarcByString(cl, dstr)
	if err != nil {
		return err
	}

	d2 := d.Copy()
	err = d2.EvolveFrom(d)
	if err != nil {
		return err
	}

	d2.Description = []byte(desc)
	d2Buf, err := d2.ToProto()
	if err != nil {
		return err
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())

	invoke := byzcoin.Invoke{
		ContractID: byzcoin.ContractDarcID,
		Command:    "evolve",
		Args: []byzcoin.Argument{
			{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID:    byzcoin.NewInstanceID(d2.GetBaseID()),
		Invoke:        &invoke,
		SignerCounter: []uint64{counters.Counters[0] + 1},
	})
	if err != nil {
		return err
	}
	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	return lib.WaitPropagation(c, cl)
}

func debugBlock(c *cli.Context) error {
	var roster *onet.Roster
	var bcID *skipchain.SkipBlockID
	var err error
	blockID, err := getIDPointer(c.String("blockID"))
	if err != nil {
		return xerrors.Errorf("couldn't get blockID: %+v", err)
	}
	blockIndex := c.Int("blockIndex")
	if blockIndex < 0 && blockID == nil {
		return errors.New("need either --index or --id")
	}
	if bcCfg := c.String("bcCfg"); bcCfg != "" {
		cfg, _, err := lib.LoadConfig(bcCfg)
		if err != nil {
			return xerrors.Errorf("couldn't get bc-config: %+v", err)
		}
		roster = &cfg.Roster
		bcID = &cfg.ByzCoinID
	}
	bcIDNew, err := getIDPointer(c.String("bcID"))
	if err != nil {
		return xerrors.Errorf("couldn't get bcID: %+v", err)
	}
	if bcIDNew != nil {
		bcID = bcIDNew
	}
	all := c.Bool("all")
	if url := c.String("url"); url != "" {
		if bcID == nil {
			return errors.New("please also give either --bcID or --bcCfg")
		}
		roster = onet.NewRoster([]*network.ServerIdentity{{
			Public: cothority.Suite.Point(),
			URL:    url,
		}})
		if all {
			sb, err := getBlock(*roster, bcID, blockID, blockIndex, 0)
			if err != nil {
				return xerrors.Errorf("couldn't get block: %+v", err)
			}
			roster = sb.Roster
		}
	}
	if roster == nil {
		return errors.New("give either --bcCfg or --url")
	}

	for i, node := range roster.List {
		url := node.URL
		if url == "" {
			url = node.Address.String()
		}
		log.Info("Contacting node", url)
		sb, err := getBlock(*roster, bcID, blockID, blockIndex, i)
		if err != nil {
			log.Warn("Got error while contacting node:", err)
			continue
		}
		var dBody byzcoin.DataBody
		err = protobuf.Decode(sb.Payload, &dBody)
		if err != nil {
			return xerrors.Errorf("couldn't decode body: %+v", err)
		}
		var dHead byzcoin.DataHeader
		err = protobuf.Decode(sb.Data, &dHead)
		if err != nil {
			return xerrors.Errorf("couldn't decode data: %+v", err)
		}
		t := time.Unix(dHead.Timestamp/1e9, 0)
		var blinks []string
		for _, l := range sb.BackLinkIDs {
			blinks = append(blinks, fmt.Sprintf("\t\tTo: %x", l))
		}
		var flinks []string
		for _, l := range sb.ForwardLink {
			flinks = append(flinks, fmt.Sprintf("\t\tTo: %x - NewRoster: %t",
				l.To, l.NewRoster != nil))
		}
		out := fmt.Sprintf("\tBlock %x (index %d) from %s\n"+
			"\tNode-list: %s\n"+
			"\tBack-links:\n%s\n"+
			"\tForward-links:\n%s\n",
			sb.Hash, sb.Index, t.String(),
			sb.Roster.List,
			strings.Join(blinks, "\n"),
			strings.Join(flinks, "\n"))
		if c.Bool("txDetails") {
			var txs []string
			for _, tx := range dBody.TxResults {
				if tx.Accepted {
					var insts []string
					for _, inst := range tx.ClientTransaction.Instructions {
						insts = append(insts, inst.String())
					}
					txs = append(txs, strings.Join(insts, "\n"))
				} else {
					txs = append(txs, "\t\tRefused TX")
				}
			}
			out += fmt.Sprintf("\tTransactions:\n%s\n",
				strings.Join(txs, "\n"))
		} else {
			out += fmt.Sprintf("\tTransactions: %d\n",
				len(dBody.TxResults))
		}
		log.Info(out)
	}

	return nil
}

func getBlock(roster onet.Roster, bcID *skipchain.SkipBlockID,
	blockID *skipchain.SkipBlockID, blockIndex int,
	node int) (*skipchain.SkipBlock, error) {
	cl := skipchain.NewClient()
	cl.UseNode(node)
	if blockID != nil {
		return cl.GetSingleBlock(&roster, *blockID)
	}
	repl, err := cl.GetSingleBlockByIndex(&roster, *bcID, blockIndex)
	if err != nil {
		return nil, xerrors.Errorf("couldn't get block: %+v", err)
	}
	return repl.SkipBlock, nil
}

func getIDPointer(s string) (*skipchain.SkipBlockID, error) {
	if s == "" {
		return nil, nil
	}
	idB, err := hex.DecodeString(s)
	if err != nil {
		return nil, xerrors.Errorf("couldn't decode %s: %+v", s, err)
	}
	idSC := skipchain.SkipBlockID(idB)
	return &idSC, nil
}

func debugList(c *cli.Context) error {
	if c.NArg() < 1 {
		return errors.New("please give (ip:port | group.toml) as argument")
	}

	var urls []string
	if f, err := os.Open(c.Args().First()); err == nil {
		defer f.Close()
		group, err := app.ReadGroupDescToml(f)
		if err != nil {
			return err
		}
		for _, si := range group.Roster.List {
			if si.URL != "" {
				urls = append(urls, si.URL)
			} else {
				p, err := strconv.Atoi(si.Address.Port())
				if err != nil {
					return err
				}
				urls = append(urls, fmt.Sprintf("http://%s:%d", si.Address.Host(), p+1))
			}
		}
	} else {
		urls = []string{c.Args().First()}
	}

	for _, url := range urls {
		log.Info("Contacting ", url)
		resp, err := byzcoin.Debug(url, nil)
		if err != nil {
			log.Error(err)
			continue
		}
		sort.SliceStable(resp.Byzcoins, func(i, j int) bool {
			var iData byzcoin.DataHeader
			var jData byzcoin.DataHeader
			err := protobuf.Decode(resp.Byzcoins[i].Genesis.Data, &iData)
			if err != nil {
				return false
			}
			err = protobuf.Decode(resp.Byzcoins[j].Genesis.Data, &jData)
			if err != nil {
				return false
			}
			return iData.Timestamp > jData.Timestamp
		})
		for _, rb := range resp.Byzcoins {
			log.Infof("ByzCoinID %x has", rb.ByzCoinID)
			headerGenesis := byzcoin.DataHeader{}
			headerLatest := byzcoin.DataHeader{}
			err := protobuf.Decode(rb.Genesis.Data, &headerGenesis)
			if err != nil {
				log.Error(err)
				continue
			}
			err = protobuf.Decode(rb.Latest.Data, &headerLatest)
			if err != nil {
				log.Error(err)
				continue
			}
			log.Infof("\tBlocks: %d\n\tFrom %s to %s\tBlock hash: %x",
				rb.Latest.Index,
				time.Unix(headerGenesis.Timestamp/1e9, 0),
				time.Unix(headerLatest.Timestamp/1e9, 0),
				rb.Latest.Hash[:])
			if c.Bool("verbose") {
				log.Infof("\tRoster: %s\n\tGenesis block header: %+v\n\tLatest block header: %+v",
					rb.Latest.Roster.List,
					rb.Genesis.SkipBlockFix,
					rb.Latest.SkipBlockFix)
			}
			log.Info()
		}
	}
	return nil
}

func debugDump(c *cli.Context) error {
	if c.NArg() < 2 {
		return errors.New("please give the following arguments: ip:port byzcoin-id")
	}

	bcidBuf, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		log.Error(err)
		return err
	}
	bcid := skipchain.SkipBlockID(bcidBuf)
	resp, err := byzcoin.Debug(c.Args().First(), &bcid)
	if err != nil {
		log.Error(err)
		return err
	}
	sort.SliceStable(resp.Dump, func(i, j int) bool {
		return bytes.Compare(resp.Dump[i].Key, resp.Dump[j].Key) < 0
	})
	for _, inst := range resp.Dump {
		log.Infof("%x / %d: %s", inst.Key, inst.State.Version, string(inst.State.ContractID))
		if c.Bool("verbose") {
			switch inst.State.ContractID {
			case byzcoin.ContractDarcID:
				d, err := darc.NewFromProtobuf(inst.State.Value)
				if err != nil {
					log.Warn("Didn't recognize as a darc instance")
				}
				log.Infof("\tDesc: %s, Rules:", string(d.Description))
				for _, r := range d.Rules.List {
					log.Infof("\tAction: %s - Expression: %s", r.Action, r.Expr)
				}
			}
		}
	}

	return nil
}

func debugRemove(c *cli.Context) error {
	if c.NArg() < 2 {
		return errors.New("please give the following arguments: private.toml byzcoin-id")
	}

	ccfg, err := app.LoadCothority(c.Args().First())
	if err != nil {
		return err
	}
	si, err := ccfg.GetServerIdentity()
	if err != nil {
		return err
	}
	bcidBuf, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		log.Error(err)
		return err
	}
	bcid := skipchain.SkipBlockID(bcidBuf)
	err = byzcoin.DebugRemove(si, bcid)
	if err != nil {
		return err
	}
	log.Infof("Successfully removed ByzCoinID %x from %s", bcid, si.Address)
	return nil
}

func debugCounters(c *cli.Context) error {
	if c.NArg() < 2 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg")
	}
	cfg, cl, signer, _, _, err := getBcKey(c)
	if err != nil {
		return err
	}
	for i, node := range cfg.Roster.List {
		if err := cl.UseNode(i); err != nil {
			return err
		}
		resp, err := cl.GetSignerCounters(signer.Identity().String())
		if err != nil {
			return err
		}
		log.Infof("node %s has counter %d", node.Address, resp.Counters[0])
	}
	return nil
}

func darcAdd(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	dstr := c.String("darc")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}
	dSpawn, err := lib.GetDarcByString(cl, dstr)
	if err != nil {
		return err
	}

	var signer *darc.Signer

	identities := c.StringSlice("identity")

	if len(identities) == 0 {
		s := darc.NewSignerEd25519(nil, nil)
		err = lib.SaveKey(s)
		if err != nil {
			return err
		}
		identities = append(identities, s.Identity().String())
	}

	Y := expression.InitParser(func(s string) bool { return true })

	for _, id := range identities {
		expr := []byte(id)
		_, err := expression.Evaluate(Y, expr)
		if err != nil {
			return errors.New("failed to parse id: " + err.Error())
		}
	}

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	var desc []byte
	if c.String("desc") == "" {
		desc = []byte(lib.RandString(10))
	} else {
		if len(c.String("desc")) > 1024 {
			return errors.New("descriptions longer than 1024 characters are not allowed")
		}
		desc = []byte(c.String("desc"))
	}

	deferredExpr := expression.InitOrExpr(identities...)
	adminExpr := expression.InitAndExpr(identities...)

	rules := darc.NewRules()
	rules.AddRule("invoke:"+byzcoin.ContractDarcID+".evolve", adminExpr)
	rules.AddRule("_sign", adminExpr)
	if c.Bool("deferred") {
		rules.AddRule("spawn:deferred", deferredExpr)
		rules.AddRule("invoke:deferred.addProof", deferredExpr)
		rules.AddRule("invoke:deferred.execProposedTx", deferredExpr)
	}
	if c.Bool("unrestricted") {
		err = rules.AddRule("invoke:"+byzcoin.ContractDarcID+".evolve_unrestricted", adminExpr)
		if err != nil {
			return err
		}
	}

	d := darc.NewDarc(rules, desc)

	dBuf, err := d.ToProto()
	if err != nil {
		return err
	}

	instID := byzcoin.NewInstanceID(dSpawn.GetBaseID())

	counters, err := cl.GetSignerCounters(signer.Identity().String())

	spawn := byzcoin.Spawn{
		ContractID: byzcoin.ContractDarcID,
		Args: []byzcoin.Argument{
			{
				Name:  "darc",
				Value: dBuf,
			},
		},
	}

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID:    instID,
		Spawn:         &spawn,
		SignerCounter: []uint64{counters.Counters[0] + 1},
	})
	if err != nil {
		return err
	}
	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.App.Writer, d.String())
	if err != nil {
		return err
	}

	// Saving ID in special file
	output := c.String("out_id")
	if output != "" {
		err = ioutil.WriteFile(output, []byte(d.GetIdentityString()), 0644)
		if err != nil {
			return err
		}
	}

	// Saving key in special file
	output = c.String("out_key")
	if len(c.StringSlice("identity")) == 0 && output != "" {
		err = ioutil.WriteFile(output, []byte(identities[0]), 0600)
		if err != nil {
			return err
		}
	}

	return lib.WaitPropagation(c, cl)
}

func darcRule(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	dstr := c.String("darc")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}
	d, err := lib.GetDarcByString(cl, dstr)
	if err != nil {
		return err
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	action := c.String("rule")
	if action == "" {
		return errors.New("--rule flag is required")
	}

	identities := c.StringSlice("identity")

	if len(identities) == 0 {
		if !c.Bool("delete") {
			return errors.New("--identity flag is required")
		}
	}

	Y := expression.InitParser(func(s string) bool { return true })

	for _, id := range identities {
		expr := []byte(id)
		_, err := expression.Evaluate(Y, expr)
		if err != nil {
			return errors.New("failed to parse id: " + err.Error())
		}
	}

	var groupExpr expression.Expr
	min := c.Uint("minimum")
	if min == 0 {
		groupExpr = expression.InitAndExpr(identities...)
	} else {
		andGroups := lib.CombinationAnds(identities, int(min))
		groupExpr = expression.InitOrExpr(andGroups...)
	}

	d2 := d.Copy()
	err = d2.EvolveFrom(d)
	if err != nil {
		return err
	}

	switch {
	case c.Bool("delete"):
		err = d2.Rules.DeleteRules(darc.Action(action))
	case c.Bool("replace"):
		err = d2.Rules.UpdateRule(darc.Action(action), groupExpr)
	default:
		err = d2.Rules.AddRule(darc.Action(action), groupExpr)
	}

	if err != nil {
		return err
	}

	d2Buf, err := d2.ToProto()
	if err != nil {
		return err
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())

	invoke := byzcoin.Invoke{
		ContractID: byzcoin.ContractDarcID,
		Command:    "evolve_unrestricted",
		Args: []byzcoin.Argument{
			{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID:    byzcoin.NewInstanceID(d2.GetBaseID()),
		Invoke:        &invoke,
		SignerCounter: []uint64{counters.Counters[0] + 1},
	})
	if err != nil {
		return err
	}
	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	return lib.WaitPropagation(c, cl)
}

// print a rule based on the identities and the minimum given.
func darcPrintRule(c *cli.Context) error {

	identities := c.StringSlice("identity")

	if len(identities) == 0 {
		if !c.Bool("delete") {
			return errors.New("--identity (-id) flag is required")
		}
	}

	Y := expression.InitParser(func(s string) bool { return true })

	for _, id := range identities {
		expr := []byte(id)
		_, err := expression.Evaluate(Y, expr)
		if err != nil {
			return errors.New("failed to parse id: " + err.Error())
		}
	}

	var groupExpr expression.Expr
	min := c.Uint("minimum")
	if min == 0 {
		groupExpr = expression.InitAndExpr(identities...)
	} else {
		andGroups := lib.CombinationAnds(identities, int(min))
		groupExpr = expression.InitOrExpr(andGroups...)
	}

	log.Infof("%s\n", groupExpr)

	return nil
}

func qrcode(c *cli.Context) error {
	type pair struct {
		Priv string
		Pub  string
	}
	type baseconfig struct {
		ByzCoinID skipchain.SkipBlockID
	}

	type adminconfig struct {
		ByzCoinID skipchain.SkipBlockID
		Admin     pair
	}

	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, _, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	var toWrite []byte

	if c.Bool("admin") {
		signer, err := lib.LoadKey(cfg.AdminIdentity)
		if err != nil {
			return err
		}

		priv, err := signer.GetPrivate()
		if err != nil {
			return err
		}

		toWrite, err = json.Marshal(adminconfig{
			ByzCoinID: cfg.ByzCoinID,
			Admin: pair{
				Priv: priv.String(),
				Pub:  signer.Identity().String(),
			},
		})
	} else {
		toWrite, err = json.Marshal(baseconfig{
			ByzCoinID: cfg.ByzCoinID,
		})
	}

	if err != nil {
		return err
	}

	qr, err := qrgo.NewQR(string(toWrite))
	if err != nil {
		return err
	}

	qr.OutputTerminal()

	return nil
}

func getInfo(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, _, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	log.Infof("BC configuration:\n"+
		"\tCongig path: %s\n"+
		"\tRoster: %s\n"+
		"\tByzCoinID: %x\n"+
		"\tDarc Base ID: %x\n"+
		"\tIdentity: %s\n",
		bcArg, cfg.Roster.List, cfg.ByzCoinID, cfg.AdminDarc.GetBaseID(), cfg.AdminIdentity.String())

	return nil
}

func resolveiid(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	ndstr := c.String("namingDarc")
	if ndstr == "" {
		ndstr = cfg.AdminDarc.GetIdentityString()
	}
	nd, err := lib.GetDarcByString(cl, ndstr)
	if err != nil {
		return err
	}

	name := c.String("name")
	if name == "" {
		return errors.New("--name flag is required")
	}

	instID, err := cl.ResolveInstanceID(nd.GetBaseID(), name)
	if err != nil {
		return errors.New("failed to resolve instance id: " + err.Error())
	}

	_, err = cl.GetProofFromLatest(instID.Slice())
	if err != nil {
		return errors.New("failed to get proof from latest: " + err.Error())
	}

	log.Infof("Here is the resolved instance id:\n%s", instID)

	return nil
}

type configPrivate struct {
	Owner darc.Signer
}

func init() { network.RegisterMessages(&configPrivate{}) }
