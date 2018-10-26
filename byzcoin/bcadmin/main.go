package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/bcadmin/lib"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/darc/expression"
	"github.com/dedis/onet"
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
		Usage:   "show the config, contact ByzCoin to get Genesis Darc ID",
		Aliases: []string{"s"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use",
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
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use",
			},
			cli.StringFlag{
				Name:  "identity",
				Usage: "the identity of the signer who will be allowed to access the contract (e.g. ed25519:a35020c70b8d735...0357))",
			},
			cli.StringFlag{
				Name:  "expression",
				Usage: "the expression that will be added to this rule",
			},
			cli.BoolFlag{
				Name:  "replace",
				Usage: "if this rule already exists, replace it with this new one",
			},
		},
		Action: add,
	},
	{
		Name:    "keys",
		Usage:   "creates a new key pair, which may be used with add",
		Aliases: []string{"k"},
		Action:  keys,
	},
}

var cliApp = cli.NewApp()

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

func init() {
	cliApp.Name = "bcadmin"
	cliApp.Usage = "Create ByzCoin ledgers and grant access to them."
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
			Value: getDataPath(cliApp.Name),
			Usage: "path to configuration-directory",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		lib.ConfigPath = c.String("config")
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
	r, err := lib.ReadRoster(fn)
	if err != nil {
		return err
	}

	interval := c.Duration("interval")

	owner := darc.NewSignerEd25519(nil, nil)

	req, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, r, []string{"spawn:darc"}, owner.Identity())
	if err != nil {
		return err
	}
	req.BlockInterval = interval

	cl := onet.NewClient(cothority.Suite, byzcoin.ServiceName)

	var resp byzcoin.CreateGenesisBlockResponse
	err = cl.SendProtobuf(r.List[0], req, &resp)
	if err != nil {
		return err
	}

	cfg := lib.Config{
		ByzCoinID:     resp.Skipblock.SkipChainID(),
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

	fmt.Fprintf(c.App.Writer, "Created ByzCoin with ID %x.\n", cfg.ByzCoinID)
	fmt.Fprintf(c.App.Writer, "export BC=\"%v\"\n", fn)

	// For the tests to use.
	c.App.Metadata["BC"] = fn

	return nil
}

func show(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	fmt.Fprintln(c.App.Writer, "ByzCoinID:", fmt.Sprintf("%x", cfg.ByzCoinID))
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
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
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

	expStr := c.String("expression")
	if expStr == "" {
		expStr = c.String("identity")
		if expStr == "" {
			return errors.New("One of --expression or --identity flag is required.")
		}
	} else {
		if c.String("identity") != "" {
			return errors.New("Only one of --expression or --identity flags allowed. Choose wisely.")
		}
	}
	exp := expression.Expr(expStr)

	d, err := cl.GetGenDarc()
	if err != nil {
		return err
	}

	d2 := d.Copy()
	d2.EvolveFrom(d)

	err = d2.Rules.AddRule(darc.Action(action), exp)
	if err != nil {
		if c.Bool("replace") {
			err = d2.Rules.UpdateRule(darc.Action(action), exp)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	d2Buf, err := d2.ToProto()
	if err != nil {
		return err
	}

	invoke := byzcoin.Invoke{
		Command: "evolve",
		Args: []byzcoin.Argument{
			byzcoin.Argument{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}
	instr := byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(d2.GetBaseID()),
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

	_, err = cl.AddTransactionAndWait(byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{instr},
	}, 10)
	if err != nil {
		return err
	}

	return nil
}

func keys(c *cli.Context) error {
	s := darc.NewSignerEd25519(nil, nil)
	fmt.Println("Identity:", s.Identity())
	fmt.Printf("export PRIVATE_KEY=%v\n", s.Ed25519.Secret)
	return nil
}
