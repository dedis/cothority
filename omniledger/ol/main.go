package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/cfgpath"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

var cmds = cli.Commands{
	{
		Name:    "create",
		Usage:   "create a ledger",
		Aliases: []string{"c"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "roster, r",
				Usage: "the roster of the cothority that will host the ledger",
			},
			cli.DurationFlag{
				Name:  "interval, i",
				Usage: "the block interval for this ledger",
				Value: 5 * time.Second,
			},
		},
		Action: create,
	},
	{
		Name:    "show",
		Usage:   "show a config",
		Aliases: []string{"s"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "ol",
				EnvVar: "OL",
				Usage:  "the OmniLedger config to use",
			},
		},
		Action: show,
	},
	{
		Name:    "add",
		Usage:   "add a rule and signer to the base darc",
		Aliases: []string{"a"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "ol",
				EnvVar: "OL",
				Usage:  "the OmniLedger config to use",
			},
			cli.StringFlag{
				Name:  "identity",
				Usage: "the identity of the signer who will be allowed to access the contract (e.g. ed25519:a35020c70b8d735...0357))",
			},
		},
		Action: add,
	},
}

var cliApp = cli.NewApp()

func init() {
	cliApp.Name = "ol"
	cliApp.Usage = "Create OmniLedger ledgers and grant access to them."
	cliApp.Version = "0.1"
	cliApp.Commands = cmds
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
}

func main() {
	log.ErrFatal(cliApp.Run(os.Args))
}

func create(c *cli.Context) error {
	fn := c.String("roster")
	if fn == "" {
		return errors.New("-roster flag is required")
	}

	in, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("Could not open roster %v: %v", fn, err)
	}
	r, err := readRoster(in)
	if err != nil {
		return err
	}

	interval := c.Duration("interval")

	owner := darc.NewSignerEd25519(nil, nil)

	req, err := omniledger.DefaultGenesisMsg(omniledger.CurrentVersion, r, []string{"spawn:darc"}, owner.Identity())
	if err != nil {
		return err
	}
	req.BlockInterval = interval

	cl := onet.NewClient(cothority.Suite, omniledger.ServiceName)

	var resp omniledger.CreateGenesisBlockResponse
	err = cl.SendProtobuf(r.List[0], req, &resp)
	if err != nil {
		return err
	}

	cfg := &config{
		ID:      resp.Skipblock.SkipChainID(),
		Roster:  *r,
		OwnerID: owner.Identity(),
	}
	fn, err = cfg.save()
	if err != nil {
		return err
	}

	cfgp := &configPrivate{
		Owner: owner,
	}
	err = cfgp.save()
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Created OmniLedger with ID %x.\n", cfg.ID)
	fmt.Fprintf(c.App.Writer, "Config file to give to clients: %v\n", fn)

	// For the tests to use.
	c.App.Metadata["OL"] = fn

	return err
}

func show(c *cli.Context) error {
	ol := c.String("ol")
	if ol == "" {
		return errors.New("-ol flag is required")
	}
	cfg, err := loadConfig(ol)
	if err != nil {
		return err
	}

	// This should NOT happen.
	if cfg.private != nil {
		return errors.New("private info stored in public file")
	}

	fmt.Fprintln(c.App.Writer, cfg)
	return nil
}

func add(c *cli.Context) error {
	ol := c.String("ol")
	if ol == "" {
		return errors.New("-ol flag is required")
	}
	cfg, err := loadConfig(ol)
	if err != nil {
		return err
	}
	cfg.loadKey()

	arg := c.Args()
	if len(arg) == 0 {
		return errors.New("need the rule to add, e.g. spawn:contractName")
	}
	action := arg[0]

	identity := c.String("identity")

	d, err := cfg.getGenDarc()
	if err != nil {
		return err
	}

	d2 := d.Copy()
	d2.EvolveFrom(d)

	d2.Rules.AddRule(darc.Action(action), []byte(identity))

	d2Buf, err := d2.ToProto()
	if err != nil {
		return err
	}

	invoke := omniledger.Invoke{
		Command: "evolve",
		Args: []omniledger.Argument{
			omniledger.Argument{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}
	instr := omniledger.Instruction{
		ObjectID: omniledger.ObjectID{
			DarcID: d2.GetBaseID(),
		},
		Index:  0,
		Length: 1,
		Invoke: &invoke,
		Signatures: []darc.Signature{
			darc.Signature{Signer: cfg.private.Owner.Identity()},
		},
	}
	err = instr.SignBy(cfg.private.Owner)
	if err != nil {
		return err
	}

	ct := omniledger.ClientTransaction{
		Instructions: []omniledger.Instruction{instr},
	}

	req := &omniledger.AddTxRequest{
		Version:     omniledger.CurrentVersion,
		SkipchainID: cfg.ID,
		Transaction: ct,
	}

	var resp omniledger.AddTxResponse
	cl := onet.NewClient(cothority.Suite, omniledger.ServiceName)
	err = cl.SendProtobuf(cfg.Roster.List[0], req, &resp)
	if err != nil {
		return err
	}

	return nil
}

type config struct {
	ID      skipchain.SkipBlockID
	Roster  onet.Roster
	OwnerID darc.Identity
	// This is not exported so it won't be written to disk.
	private *configPrivate
}

type configPrivate struct {
	Owner darc.Signer
}

func init() {
	network.RegisterMessages(&config{}, &configPrivate{})
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

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

func loadConfig(fn string) (*config, error) {
	buf, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	_, val, err := network.Unmarshal(buf, cothority.Suite)
	if err != nil {
		return nil, err
	}
	if cfg, ok := val.(*config); ok {
		return cfg, nil
	}

	return nil, errors.New("unexpected config format")
}

func (c *config) loadKey() {
	// Find private key file.
	cfgDir := getDataPath(cliApp.Name)
	fn := fmt.Sprintf("key-%x.cfg", c.OwnerID)
	fn = filepath.Join(cfgDir, fn)

	// This might fail, no problem. Just cannot use tools that need to sign.
	p, _ := loadPrivate(fn)
	c.private = p

	return
}

func loadPrivate(fn string) (*configPrivate, error) {
	buf, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	_, val, err := network.Unmarshal(buf, cothority.Suite)
	if err != nil {
		return nil, err
	}
	if cfg, ok := val.(*configPrivate); ok {
		return cfg, nil
	}
	return nil, errors.New("unexpected private config format")
}

func (cfg *config) save() (string, error) {
	cfgDir := getDataPath(cliApp.Name)
	os.MkdirAll(cfgDir, 0755)

	fn := fmt.Sprintf("%x.cfg", cfg.ID[0:8])
	fn = filepath.Join(cfgDir, fn)

	buf, err := network.Marshal(cfg)
	if err != nil {
		return fn, err
	}
	err = ioutil.WriteFile(fn, buf, 0644)
	if err != nil {
		return fn, err
	}

	return fn, nil
}

func (cfg *configPrivate) save() error {
	cfgDir := getDataPath(cliApp.Name)
	os.MkdirAll(cfgDir, 0755)

	fn := fmt.Sprintf("key-%x.cfg", cfg.Owner.Identity())
	fn = filepath.Join(cfgDir, fn)

	// perms = 0400 because there is key material inside this file.
	f, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0400)
	if err != nil {
		return fmt.Errorf("could not write %v: %v", fn, err)
	}

	buf, err := network.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = f.Write(buf)
	if err != nil {
		return err
	}
	return f.Close()
}

// getGenDarc uses omniledger's GetProof method to fetch the latest version of the darc
// from OmniLedger and parses it.
func (c *config) getGenDarc() (*darc.Darc, error) {
	cl := omniledger.NewClient()
	p, err := cl.GetProof(&c.Roster, c.ID, omniledger.GenesisReferenceID.Slice())
	if err != nil {
		return nil, err
	}
	if !p.Proof.InclusionProof.Match() {
		return nil, errors.New("cannot find genesis Darc ID")
	}

	_, vs, err := p.Proof.KeyValue()

	if len(vs) < 2 {
		return nil, errors.New("not enough records")
	}
	contractBuf := vs[1]
	if string(contractBuf) != "config" {
		return nil, errors.New("expected contract to be config but got: " + string(contractBuf))
	}
	darcBuf := vs[0]
	if len(darcBuf) != 32 {
		return nil, errors.New("genesis darc ID is wrong length")
	}

	p, err = cl.GetProof(&c.Roster, c.ID, omniledger.ObjectID{DarcID: darcBuf}.Slice())
	if err != nil {
		return nil, err
	}
	if !p.Proof.InclusionProof.Match() {
		return nil, errors.New("cannot find genesis Darc")
	}

	_, vs, err = p.Proof.KeyValue()

	if len(vs) < 2 {
		return nil, errors.New("not enough records")
	}
	contractBuf = vs[1]
	if string(contractBuf) != "darc" {
		return nil, errors.New("expected contract to be darc but got: " + string(contractBuf))
	}
	d, err := darc.NewDarcFromProto(vs[0])
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (c *config) String() string {
	var r []string
	for _, x := range c.Roster.List {
		r = append(r, x.Address.NetworkAddress())
	}

	return fmt.Sprintf("ID: %x\nRoster: %v", c.ID, strings.Join(r, ", "))
}

func rosterToServers(r *onet.Roster) []network.Address {
	out := make([]network.Address, len(r.List))
	for i := range r.List {
		out[i] = r.List[i].Address
	}
	return out
}
