/*
* This is a template for creating an app. It only has one command which
* prints out the name of the app.
 */
package main

import (
	"errors"
	"fmt"
	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/bcadmin/lib"
	"github.com/dedis/cothority/byzcoin/darc"
	"github.com/dedis/cothority/omniledger"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
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
	IBDarc        darc.Darc
	ShardIDs      []skipchain.SkipBlockID
	ShardRosters  []onet.Roster
	ShardDarcs    []darc.Darc
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
			Name:      "evolve",
			Usage:     "evolves a shard",
			ArgsUsage: "the omniledger config, the key config and the roster files",
			Action:    evolveShard,
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
	/*fmt.Println(c.NArg())
	if c.NArg() < 5 { // NArg() counts the flags and their value as arguments
		return errors.New("Not enough arguments (1 required)")
	}*/

	// Parse CLI arguments
	sn := c.Int("shards")
	if sn == 0 {
		return errors.New(`--shards flag is required 
			and its value must be greater than 0`)
	}

	es := time.Duration(c.Int("epoch")) * time.Millisecond
	if es == 0 {
		return errors.New(`--epoch flag is required 
			and its value must be greather than 0`)
	}

	// Parse, open and read roster file
	rosterPath := c.Args().First()
	fp, err := os.Open(rosterPath)
	if err != nil {
		return fmt.Errorf("Could not open roster %v, %v", rosterPath, err)
	}
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
		ShardRosters:  reply.ShardRoster,
		IBID:          reply.IDSkipBlock.SkipChainID(),
		ShardIDs:      shardIDs,
		IBDarc:        reply.GenesisDarc,
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

	// For testing purposes
	c.App.Metadata["OC"] = cfgPath

	return nil
}

func evolveShard(c *cli.Context) error {
	if c.NArg() < 3 {
		return errors.New("Not enough arguments (3 required")
	}

	/*
		olPath := c.Args().Get(0)
		keyPath := c.Args().Get(1)
		rosterPath := c.Args().Get(2)
	*/

	return nil
}

// TODO: Finish function
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
		IBDarcID:     cfg.IBDarc,
		ShardIDs:     cfg.ShardIDs,
		ShardDarcIDs: cfg.ShardDarcs,
		ShardRosters: cfg.ShardRosters,
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

	cfg.IBRoster = rep.IBRoster
	cfg.ShardRosters = rep.ShardRosters

	cfgPath, err = saveConfig(*cfg)
	if err != nil {
		return err
	}

	return nil
}

// TODO: Finish function
func darcUpdate(c *cli.Context) error {

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
	var cfgBuf []byte
	cfgBuf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	err = protobuf.Decode(cfgBuf, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
