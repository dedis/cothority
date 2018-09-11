package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/omniledger/ol/lib"
	ol "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/cfgpath"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	cli "gopkg.in/urfave/cli.v1"
)

func init() {
	network.RegisterMessages(&darc.Darc{}, &darc.Identity{}, &darc.Signer{})
}

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
		Usage:   "show the config, contact OmniLedger to get Genesis Darc ID",
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

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

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
		cli.StringFlag{
			Name:  "config, c",
			Value: "",
			Usage: "path to configuration-directory",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		lib.ConfigPath = c.String("config")
		if lib.ConfigPath == "" {
			lib.ConfigPath = getDataPath(cliApp.Name)
		}
		return nil
	}
}

func main() {
	log.ErrFatal(cliApp.Run(os.Args))
}

func create(c *cli.Context) error {
	fn := c.String("roster")
	if fn == "" {
		return errors.New("--roster flag is required")
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

	req, err := ol.DefaultGenesisMsg(ol.CurrentVersion, r, []string{"spawn:darc"}, owner.Identity())
	if err != nil {
		return err
	}
	req.BlockInterval = interval

	cl := onet.NewClient(cothority.Suite, ol.ServiceName)

	var resp ol.CreateGenesisBlockResponse
	err = cl.SendProtobuf(r.List[0], req, &resp)
	if err != nil {
		return err
	}

	cfg := lib.Config{
		OmniledgerID:  resp.Skipblock.SkipChainID(),
		Roster:        *r,
		GenesisDarc:   req.GenesisDarc,
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

	fmt.Fprintf(c.App.Writer, "Created OmniLedger with ID %x.\n", cfg.OmniledgerID)
	fmt.Fprintf(c.App.Writer, "export OL=\"%v\"\n", fn)

	// For the tests to use.
	c.App.Metadata["OL"] = fn

	return nil
}

func show(c *cli.Context) error {
	olArg := c.String("ol")
	if olArg == "" {
		return errors.New("--ol flag is required")
	}

	cfg, cl, err := lib.LoadConfig(olArg)
	if err != nil {
		return err
	}

	fmt.Fprintln(c.App.Writer, cfg)

	fmt.Fprintln(c.App.Writer)
	fmt.Fprintln(c.App.Writer, "Genesis Darc:")
	var roster []string
	for _, s := range cfg.Roster.List {
		roster = append(roster, string(s.Address))
	}
	fmt.Fprintln(c.App.Writer, "Roster:", strings.Join(roster, ", "))

	gd, err := cl.GetGenDarc()
	if err == nil {
		fmt.Fprintln(c.App.Writer, gd)
	} else {
		fmt.Fprintln(c.App.ErrWriter, "could not fetch darc:", err)
	}

	return err
}

func add(c *cli.Context) error {
	olArg := c.String("ol")
	if olArg == "" {
		return errors.New("--ol flag is required")
	}

	cfg, cl, err := lib.LoadConfig(olArg)
	if err != nil {
		return err
	}

	signer, err := lib.LoadKey(cfg.AdminIdentity)
	if err != nil {
		return err
	}

	arg := c.Args()
	if len(arg) == 0 {
		return errors.New("need the rule to add, e.g. spawn:contractName")
	}
	action := arg[0]

	identity := c.String("identity")
	if identity == "" {
		return errors.New("--identity flag is required")
	}

	d, err := cl.GetGenDarc()
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

	invoke := ol.Invoke{
		Command: "evolve",
		Args: []ol.Argument{
			ol.Argument{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}
	instr := ol.Instruction{
		InstanceID: ol.NewInstanceID(d2.GetBaseID()),
		Index:      0,
		Length:     1,
		Invoke:     &invoke,
		Signatures: []darc.Signature{
			darc.Signature{Signer: signer.Identity()},
		},
	}
	err = instr.SignBy(d2.GetBaseID(), *signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ol.ClientTransaction{
		Instructions: []ol.Instruction{instr},
	}, 10)
	if err != nil {
		return err
	}

	return nil
}

type configPrivate struct {
	Owner darc.Signer
}

func init() { network.RegisterMessages(&configPrivate{}) }

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

func rosterToServers(r *onet.Roster) []network.Address {
	out := make([]network.Address, len(r.List))
	for i := range r.List {
		out[i] = r.List[i].Address
	}
	return out
}
