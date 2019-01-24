package main

import (
	"errors"
	"fmt"
	"github.com/dedis/cothority"
	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/bcadmin/lib"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/omniledger"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	"gopkg.in/urfave/cli.v1"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// ConfigPath points to where the files will be stored by default.
var ConfigPath = "."

// Config is the structure used to save an omniledger (identity ledger and shards) configuration.
type Config struct {
	IBID          skipchain.SkipBlockID
	IBRoster      onet.Roster
	ShardIDs      []skipchain.SkipBlockID
	AdminIdentity darc.Identity
	OLInstanceID  bc.InstanceID
}

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "oladmin"
	cliApp.Usage = "Handles sharding of transactions"
	cliApp.Version = "0.1"
	cliApp.Commands = []cli.Command{
		{
			Name:      "create",
			Usage:     "creates the sharding",
			ArgsUsage: "the roster file",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "shards",
					Usage: "the number of shards on which the ledger will be partitioned",
				},
				cli.IntFlag{
					Name:  "epoch",
					Usage: "the epoch-size in millisecond",
				},
			},
			Action: createSharding,
		},
		{
			Name:      "status",
			Usage:     "prints the current roster and shard rosters",
			ArgsUsage: "the omniledger config file",
			Action:    getStatus,
		},
		{
			Name:      "newepoch",
			Usage:     "starts a new epoch",
			ArgsUsage: "the omniledger config and the key config files",
			Action:    newEpoch,
		},
	}
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	log.ErrFatal(cliApp.Run(os.Args))
}

func createSharding(c *cli.Context) error {
	// Parse CLI arguments
	sn := c.Int("shards")
	if sn == 0 {
		return errors.New(`--shards flag is required 
			and its value must be greater than 0`)
	}

	es := time.Duration(c.Int("epoch")) * time.Millisecond
	if es == 0 {
		return errors.New(`--epoch flag is required 
			and its value must be greater than 0`)
	}

	// Parse, open and read roster file
	rosterPath := c.Args().First()
	fp, err := os.Open(rosterPath)
	if err != nil {
		return fmt.Errorf("Could not open roster %v, %v", rosterPath, err)
	}
	defer fp.Close()

	roster, err := readRoster(fp)
	if err != nil {
		return err
	}

	// Check #shard is not too high
	if 4*sn > len(roster.List) {
		return fmt.Errorf("Not enough validators per shard, there should be at least 4 validators per shard")
	}

	// Create request and reply struct
	req := &omniledger.CreateOmniLedger{
		Roster:     *roster,
		ShardCount: sn,
		EpochSize:  es,
		Timestamp:  time.Now(),
		Version:    bc.CurrentVersion,
	}

	// Create new omniledger
	_, reply, err := omniledger.NewOmniLedger(req)
	if err != nil {
		return err
	}

	// Create config
	shardIDs := make([]skipchain.SkipBlockID, len((*reply).ShardBlocks))
	for i := 0; i < len(shardIDs); i++ {
		shardIDs[i] = ((*reply).ShardBlocks)[i].SkipChainID()
	}

	cfg := Config{
		IBRoster:      *roster,
		IBID:          reply.IDSkipBlock.SkipChainID(),
		ShardIDs:      shardIDs,
		AdminIdentity: reply.Owner.Identity(),
		OLInstanceID:  reply.OmniledgerInstanceID,
	}

	// Save config and keys
	cfgPath, err := saveConfig(cfg)
	if err != nil {
		return err
	}

	if err = lib.SaveKey(reply.Owner); err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Created OmniLedger with ID %x. \n", cfg.IBID)
	fmt.Fprintf(c.App.Writer, "export OC=\"%v\"\n", cfgPath)

	return nil
}

func newEpoch(c *cli.Context) error {
	if c.NArg() < 2 {
		return errors.New("Not enough arguments (2 required")
	}

	// Parse CLI arguments
	cfgPath := c.Args().Get(0)
	keyPath := c.Args().Get(1)

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return err
	}

	signer, err := lib.LoadSigner(keyPath)
	if err != nil {
		return err
	}

	req := &omniledger.NewEpoch{
		IBID:         cfg.IBID,
		IBRoster:     cfg.IBRoster,
		ShardIDs:     cfg.ShardIDs,
		Owner:        *signer,
		OLInstanceID: cfg.OLInstanceID,
		Timestamp:    time.Now(),
	}

	// Connect to an OL client and request new epoch
	client := omniledger.NewClient(cfg.IBID, cfg.IBRoster)
	rep, err := client.NewEpoch(req)
	if err != nil {
		return err
	}

	// Save the IB roster resulting from the new epoch
	cfg.IBRoster = rep.IBRoster

	cfgPath, err = saveConfig(*cfg)
	if err != nil {
		return err
	}

	return nil
}

func getStatus(c *cli.Context) error {
	// Parse CLI arguments
	cfgPath := c.Args().Get(0)

	// Load config file
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return err
	}

	// Prepare request
	req := &omniledger.GetStatus{
		IBID:         cfg.IBID,
		IBRoster:     cfg.IBRoster,
		OLInstanceID: cfg.OLInstanceID,
	}

	// Connect to client, send request and get reply
	client := omniledger.NewClient(cfg.IBID, cfg.IBRoster)
	reply, err := client.GetStatus(req)
	if err != nil {
		return err
	}

	// Print results
	fmt.Fprintln(c.App.Writer, "Omniledger roster:", reply.IBRoster.List)
	for ind, sr := range reply.ShardRosters {
		fmt.Fprintln(c.App.Writer, "Shard "+fmt.Sprint(ind)+" roster:", sr.List)
	}

	return nil
}

func readRoster(r io.Reader) (*onet.Roster, error) {
	group, err := app.ReadGroupDescToml(r)
	if err != nil {
		return nil, err
	}

	if len(group.Roster.List) == 0 {
		return nil, errors.New("empty roster")
	}
	return group.Roster, nil
}

func saveConfig(cfg Config) (string, error) {
	os.MkdirAll(ConfigPath, 0755)

	fn := fmt.Sprintf("ol-%x.cfg", cfg.IBID)
	fn = filepath.Join(ConfigPath, fn)

	buf, err := protobuf.Encode(&cfg)
	if err != nil {
		return fn, err
	}
	err = ioutil.WriteFile(fn, buf, 0644)
	if err != nil {
		return fn, err
	}

	return fn, nil
}

func loadConfig(file string) (*Config, error) {
	cfg := &Config{}

	var cfgBuf []byte
	cfgBuf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	err = protobuf.DecodeWithConstructors(cfgBuf, cfg,
		network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
