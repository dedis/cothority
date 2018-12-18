package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/bcadmin/lib"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/darc/expression"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/cfgpath"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"

	"encoding/json"
	qrgo "github.com/skip2/go-qrcode"
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
		Name:    "key",
		Usage:   "generates a new keypair and prints the public key in the stdout",
		Aliases: []string{"k"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "save",
				Usage: "file in which the user wants to save the public key instead of printing it",
			},
		},
		Action: key,
	},
	{
		Name: "darc",
		Usage: "tool used to manage darcs: it can be used with multiple subcommands (add, show, rule)\n" +
			"add : adds a new DARC with specified characteristics\n" +
			"show: shows the specified DARC\n" +
			"rule: allow to add, update or delete a rule of the DARC",
		Aliases: []string{"d"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use (always use)",
			},
			cli.StringFlag{
				Name:  "owner",
				Usage: "owner of the darc allowed to sign and evolve it (eventually use with add ; default is random)",
			},
			cli.StringFlag{
				Name:  "darc",
				Usage: "darc from which we create the new darc - genesis if not mentioned (eventually use with add, rule or show ; default is Genesis DARC)",
			},
			cli.StringFlag{
				Name:  "sign",
				Usage: "public key of the signing entity - it should have been generated using this bcadmin xonfig so that it can be retrieved in local files (eventually use with add or rule ; default is admin identity)",
			},
			cli.StringFlag{
				Name:  "identity",
				Usage: "the identity of the signer who will be allowed to access the contract (e.g. ed25519:a35020c70b8d735...0357) (always use with rule, except if deleting))",
			},
			cli.StringFlag{
				Name:  "rule",
				Usage: "the rule to be added, updated or deleted (always use with rule)",
			},
			cli.StringFlag{
				Name:  "out",
				Usage: "output file for the whole darc description (eventually use with add or show)",
			},
			cli.StringFlag{
				Name:  "out_id",
				Usage: "output file for the darc id (eventually use with add)",
			},
			cli.StringFlag{
				Name:  "out_key",
				Usage: "output file for the darc key (eventually use with add)",
			},
			cli.BoolFlag{
				Name:  "replace",
				Usage: "if this rule already exists, replace it with this new one (eventually use with rule)",
			},
			cli.BoolFlag{
				Name:  "delete",
				Usage: "if this rule already exists, delete the rule (eventually use with rule)",
			},
		},
		Action: darcCli,
	},
	{
		Name:    "qrcode",
		Usage:   "generates a QRCode containing the description of the BC Config",
		Aliases: []string{"qr"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config to use (always use)",
			},
			cli.StringFlag{
				Name:  "out",
				Usage: "the png file in which the QR code is saved",
			},
			cli.BoolFlag{
				Name:  "admin",
				Usage: "If specified, the QR Code will contain the admin keypair",
			},
		},
		Action: qrcode,
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
			return errors.New("one of --expression or --identity flag is required")
		}
	} else {
		if c.String("identity") != "" {
			return errors.New("only one of --expression or --identity flags allowed, choose wisely")
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

	signatureCtr, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return err
	}
	if len(signatureCtr.Counters) != 1 {
		return errors.New("invalid result from GetSignerCounters")
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
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{
			{
				InstanceID: byzcoin.NewInstanceID(d2.GetBaseID()),
				Invoke:     &invoke,
				Signatures: []darc.Signature{
					darc.Signature{Signer: signer.Identity()},
				},
				SignerCounter: []uint64{signatureCtr.Counters[0] + 1},
			},
		},
	}
	err = ctx.SignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	return nil
}

func key(c *cli.Context) error {
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
		defer file.Close()
	}
	fmt.Fprintln(fo, newSigner.Identity().String())
	return nil
}

func darcCli(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	arg := c.Args()
	if len(arg) == 0 {
		arg = append(arg, "show")
	}

	var d *darc.Darc

	dstr := c.String("darc")
	if dstr == "" {
		d, err = cl.GetGenDarc()
		if err != nil {
			return err
		}
	} else {
		d, err = getDarcByString(cl, dstr)
		if err != nil {
			return err
		}
	}

	switch arg[0] {
	case "show":
		return darcShow(c, d)
	case "add":
		return darcAdd(c, d, cfg, cl)
	case "rule":
		return darcRule(c, d, c.Bool("replace"), c.Bool("delete"), cfg, cl)
	default:
		return errors.New("Invalid argument for darc command : add, show and rule are the valid options")
	}
}

func darcAdd(c *cli.Context, dGen *darc.Darc, cfg lib.Config, cl *byzcoin.Client) error {
	var signer *darc.Signer
	var err error

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
		if err != nil {
			return err
		}
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
		if err != nil {
			return err
		}
	}

	var identity darc.Identity
	var newSigner darc.Signer

	owner := c.String("owner")
	if owner != "" {
		tmpSigner, err := lib.LoadKeyFromString(owner)
		if err != nil {
			return err
		}
		newSigner = *tmpSigner
		identity = newSigner.Identity()
	} else {
		newSigner = darc.NewSignerEd25519(nil, nil)
		lib.SaveKey(newSigner)
		identity = newSigner.Identity()
	}

	rules := darc.InitRulesWith([]darc.Identity{identity}, []darc.Identity{identity}, "invoke:evolve")
	d := darc.NewDarc(rules, random.Bits(32, true, random.New()))

	dBuf, err := d.ToProto()
	if err != nil {
		return err
	}

	instID := byzcoin.NewInstanceID(dGen.GetBaseID())

	counters, err := cl.GetSignerCounters(signer.Identity().String())

	spawn := byzcoin.Spawn{
		ContractID: "darc",
		Args: []byzcoin.Argument{
			byzcoin.Argument{
				Name:  "darc",
				Value: dBuf,
			},
		},
	}
	instr := byzcoin.Instruction{
		InstanceID:    instID,
		Spawn:         &spawn,
		SignerCounter: []uint64{counters.Counters[0] + 1},
	}
	ctx, err := combineInstrsAndSign(*signer, instr)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	fmt.Println(d.String())

	// Saving ID in special file
	output := c.String("out_id")
	if output != "" {
		fo, err := os.Create(output)
		if err != nil {
			panic(err)
		}

		fo.Write([]byte(d.GetIdentityString()))

		fo.Close()
	}

	// Saving key in special file
	output = c.String("out_key")
	if output != "" {
		fo, err := os.Create(output)
		if err != nil {
			panic(err)
		}

		fo.Write([]byte(newSigner.Identity().String()))

		fo.Close()
	}

	// Saving description in special file
	output = c.String("out")
	if output != "" {
		fo, err := os.Create(output)
		if err != nil {
			panic(err)
		}

		fo.Write([]byte(d.String()))

		fo.Close()
	}

	return nil
}

func darcShow(c *cli.Context, d *darc.Darc) error {
	output := c.String("out")
	if output != "" {
		fo, err := os.Create(output)
		if err != nil {
			panic(err)
		}

		fo.Write([]byte(d.String()))

		fo.Close()
	} else {
		fmt.Println(d.String())
	}

	return nil
}

func darcRule(c *cli.Context, d *darc.Darc, update bool, delete bool, cfg lib.Config, cl *byzcoin.Client) error {
	var signer *darc.Signer
	var err error

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
		if err != nil {
			return err
		}
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
		if err != nil {
			return err
		}
	}

	action := c.String("rule")
	if action == "" {
		return errors.New("--rule flag is required")
	}

	if delete {
		return darcRuleDel(c, d, action, signer, cl)
	}

	identity := c.String("identity")
	if identity == "" {
		return errors.New("--identity flag is required")
	}

	d2 := d.Copy()
	d2.EvolveFrom(d)

	if update {
		err = d2.Rules.UpdateRule(darc.Action(action), []byte(identity))
	} else {
		err = d2.Rules.AddRule(darc.Action(action), []byte(identity))
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
		Command: "evolve",
		Args: []byzcoin.Argument{
			byzcoin.Argument{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}
	instr := byzcoin.Instruction{
		InstanceID:    byzcoin.NewInstanceID(d2.GetBaseID()),
		Invoke:        &invoke,
		SignerCounter: []uint64{counters.Counters[0] + 1},
	}
	ctx, err := combineInstrsAndSign(*signer, instr)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	return nil
}

func darcRuleDel(c *cli.Context, d *darc.Darc, action string, signer *darc.Signer, cl *byzcoin.Client) error {
	var err error

	d2 := d.Copy()
	d2.EvolveFrom(d)

	err = d2.Rules.DeleteRules(darc.Action(action))
	if err != nil {
		return err
	}

	d2Buf, err := d2.ToProto()
	if err != nil {
		return err
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())

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
		InstanceID:    byzcoin.NewInstanceID(d2.GetBaseID()),
		Invoke:        &invoke,
		SignerCounter: []uint64{counters.Counters[0] + 1},
	}
	ctx, err := combineInstrsAndSign(*signer, instr)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	return nil
}

func qrcode(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	out := c.String("out")
	if out == "" {
		return errors.New("--out flag is required")
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
		sig, err := signer.String()
		toWrite, err = json.Marshal(lib.AdminConfig{cfg.ByzCoinID, sig})
	} else {
		toWrite, err = json.Marshal(lib.BaseConfig{cfg.ByzCoinID})
	}

	if err != nil {
		return err
	}

	err = qrgo.WriteFile(string(toWrite), qrgo.Low, 1024, out)
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

func getDarcByString(cl *byzcoin.Client, id string) (*darc.Darc, error) {
	var xrep []byte
	fmt.Sscanf(id[5:], "%x", &xrep)
	return getDarcByID(cl, xrep)
}

func getDarcByID(cl *byzcoin.Client, id []byte) (*darc.Darc, error) {
	pr, err := cl.GetProof(id)
	if err != nil {
		return nil, err
	}

	p := &pr.Proof
	var vs []byte
	_, vs, _, _, err = p.KeyValue()
	if err != nil {
		return nil, err
	}

	d, err := darc.NewFromProtobuf(vs)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func combineInstrsAndSign(signer darc.Signer, instrs ...byzcoin.Instruction) (byzcoin.ClientTransaction, error) {
	t := byzcoin.ClientTransaction{
		Instructions: instrs,
	}
	h := t.Instructions.Hash()
	for i := range t.Instructions {
		if err := t.Instructions[i].SignWith(h, signer); err != nil {
			return t, err
		}
	}
	return t, nil
}
