package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/cothority/v3/personhood"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"

	"github.com/urfave/cli"
)

func init() {
	network.RegisterMessages(&darc.Darc{}, &darc.Identity{}, &darc.Signer{})
}

var spawnerFlags = cli.FlagsByName{
	cli.Uint64Flag{
		Name:  "darc",
		Usage: "number of coins needed to spawn a darc",
		Value: 100,
	},
	cli.Uint64Flag{
		Name:  "coin",
		Usage: "number of coins needed to spawn a coin",
		Value: 100,
	},
	cli.Uint64Flag{
		Name:  "credential",
		Usage: "number of coins needed to spawn a credential",
		Value: 100,
	},
	cli.Uint64Flag{
		Name:  "party",
		Usage: "number of coins needed to spawn a party",
		Value: 1e7,
	},
	cli.Uint64Flag{
		Name:  "rps",
		Usage: "number of coins needed to spawn a rock-paper-scissors game",
		Value: 0,
	},
}

var cmds = cli.Commands{
	{
		Name:      "spawner",
		Usage:     "create a new spawner-instance",
		Aliases:   []string{"sp"},
		ArgsUsage: "bc-xxx.cfg key-xxx.cfg",
		Action:    spawner,
		Flags:     spawnerFlags,
	},
	{
		Name:      "spawnerUpdate",
		Usage:     "update an existing spawner-instance",
		Aliases:   []string{"su"},
		ArgsUsage: "bc-xxx.cfg key-xxx.cfg spawnIID",
		Action:    spawnerUpdate,
		Flags:     spawnerFlags,
	},
	{
		Name:      "wipeParties",
		Usage:     "wipe all cached parties",
		Aliases:   []string{"wp"},
		ArgsUsage: "bc-xxx.cfg",
		Action:    wipeParties,
	},
	{
		Name:      "wipeRPS",
		Usage:     "wipe all cached RockPaperScissors games",
		Aliases:   []string{"wr"},
		ArgsUsage: "bc-xxx.cfg",
		Action:    wipeRPS,
	},
	{
		Name:      "register",
		Usage:     "register a new user",
		Aliases:   []string{"r"},
		ArgsUsage: "bc-xxx.cfg key-xxx.cfg \"http://...\"",
		Action:    register,
	},
	{
		Name:      "show",
		Usage:     "show all credentials of a user",
		Aliases:   []string{"s"},
		ArgsUsage: "bc-xxx.cfg credentialIID",
		Action:    show,
	},
}

var cliApp = cli.NewApp()

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

func init() {
	cliApp.Name = "phapp"
	cliApp.Usage = "Register users and show their credentials"
	cliApp.Version = "0.1"
	cliApp.Commands = cmds
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.BoolFlag{
			Name:   "wait, w",
			EnvVar: "BC_WAIT",
			Usage:  "wait for transaction available in all nodes",
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

func spawnerUpdate(c *cli.Context) error {
	if c.NArg() != 3 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg spawnIID")
	}

	cfg, cl, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return err
	}
	signer, err := lib.LoadSigner(c.Args().Get(1))
	if err != nil {
		return err
	}
	err = verifyAdminDarc(cl, cfg, *signer)
	if err != nil {
		return err
	}
	spIIDBuf, err := hex.DecodeString(c.Args().Get(2))
	if err != nil {
		return err
	}

	var args byzcoin.Arguments
	for _, cn := range []string{"Darc", "Coin", "Credential", "Party", "RoPaSci"} {
		coin := &byzcoin.Coin{
			Name:  personhood.SpawnerCoin,
			Value: c.Uint64(strings.ToLower(cn)),
		}
		buf, err := protobuf.Encode(coin)
		if err != nil {
			return err
		}
		args = append(args, byzcoin.Argument{
			Name:  "cost" + cn,
			Value: buf,
		})
	}
	ctx, err := combineInstrsAndSign(cl, *signer, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(spIIDBuf),
		Invoke: &byzcoin.Invoke{
			ContractID: personhood.ContractCredentialID,
			Command:    "update",
			Args:       args,
		},
	})
	if err != nil {
		return err
	}
	log.Infof("Updating Spawner instance: %x", spIIDBuf)
	_, err = cl.AddTransactionAndWait(ctx, 5)
	if err != nil {
		return err
	}
	return nil
}

func spawner(c *cli.Context) error {
	if c.NArg() != 2 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg")
	}

	cfg, cl, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return err
	}
	signer, err := lib.LoadSigner(c.Args().Get(1))
	if err != nil {
		return err
	}
	err = verifyAdminDarc(cl, cfg, *signer)
	if err != nil {
		return err
	}

	var args byzcoin.Arguments
	for _, cn := range []string{"Darc", "Coin", "Credential", "Party"} {
		coin := &byzcoin.Coin{
			Name:  personhood.SpawnerCoin,
			Value: c.Uint64(strings.ToLower(cn)),
		}
		buf, err := protobuf.Encode(coin)
		if err != nil {
			return err
		}
		args = append(args, byzcoin.Argument{
			Name:  "cost" + cn,
			Value: buf,
		})
	}
	ctx, err := combineInstrsAndSign(cl, *signer, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(cfg.AdminDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: personhood.ContractSpawnerID,
			Args:       args,
		},
	})
	if err != nil {
		return err
	}
	log.Printf("%+v", ctx.Instructions[0])
	spawnerIID := ctx.Instructions[0].DeriveID("")
	log.Infof("Creating Spawner instance: %x", spawnerIID.Slice())
	_, err = cl.AddTransactionAndWait(ctx, 5)
	if err != nil {
		return err
	}

	log.Infof("For Defaults.ts:")
	log.Infof("    ByzCoinID: Buffer.from(\"%x\", 'hex'),", cfg.ByzCoinID)
	log.Infof("    SpawnerIID: new InstanceID(Buffer.from(\"%x\", 'hex')),", spawnerIID.Slice())
	return nil
}

func wipeParties(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("please give the following argument: bc-xxx.cfg")
	}

	cfg, _, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return err
	}
	cl := personhood.NewClient()
	log.Info("Wiping all stored parties")
	errs := cl.WipeParties(cfg.Roster)
	return errsToErr(errs)
}

func wipeRPS(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("please give the following argument: bc-xxx.cfg")
	}

	cfg, _, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return err
	}
	cl := personhood.NewClient()
	log.Info("Wiping all stored Rock Paper Scissors games")
	errs := cl.WipeRoPaScis(cfg.Roster)
	return errsToErr(errs)
}

func errsToStrs(errs []error) (errsStr []string) {
	for _, err := range errs {
		errsStr = append(errsStr, err.Error())
	}
	return errsStr
}
func errsToStr(errs []error) string {
	return strings.Join(errsToStrs(errs), " -- ")
}
func errsToErr(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errors.New(errsToStr(errs))
}

func register(c *cli.Context) error {
	if c.NArg() != 3 {
		return errors.New("please give the following arguments: bc-xxx.cfg key-xxx.cfg \"http://...\"")
	}

	cfg, cl, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return err
	}
	signer, err := lib.LoadSigner(c.Args().Get(1))
	if err != nil {
		return err
	}
	err = verifyAdminDarc(cl, cfg, *signer)
	if err != nil {
		return err
	}

	urlArgs := strings.SplitN(c.Args().Get(2), "?", 2)
	if urlArgs[0] != "https://pop.dedis.ch/qrcode/unregistered-1" || len(urlArgs) != 2 {
		return errors.New("this is not an unregistered argument")
	}
	cgiArgs := strings.Split(urlArgs[1], "&")
	if len(cgiArgs) != 2 {
		return errors.New("these are not the correct arguments")
	}
	pub := cothority.Suite.Point()
	var pubBuf []byte
	var alias string
	for _, cgiArg := range cgiArgs {
		kv := strings.Split(cgiArg, "=")
		if len(kv) != 2 {
			return errors.New("wrong arguments")
		}
		switch kv[0] {
		case "public_ed25519":
			pubBuf, err = hex.DecodeString(kv[1])
			if err != nil {
				return errors.New("couldn't decode public key hex: " + err.Error())
			}
			err = pub.UnmarshalBinary(pubBuf)
			if err != nil {
				return errors.New("couldn't decode public key: " + err.Error())
			}
		case "alias":
			alias = string(kv[1])
		}
	}

	gdID := byzcoin.NewInstanceID(cfg.AdminDarc.GetBaseID())
	id := darc.NewIdentityEd25519(pub)
	rules := darc.InitRulesWith([]darc.Identity{id}, []darc.Identity{id}, "invoke:"+byzcoin.ContractDarcID+".evolve")
	expr := id.String()
	for _, s := range []string{personhood.ContractCredentialID + ".update", contracts.ContractCoinID + ".fetch", contracts.ContractCoinID + ".transfer"} {
		rules.AddRule(darc.Action("invoke:"+s), expression.Expr(expr))
	}
	d := darc.NewDarc(rules, []byte("user "+alias))
	dBuf, err := d.ToProto()
	if err != nil {
		return err
	}
	log.Infof("Darc is: %+v", d)
	log.Infof("Creating Darc for user: %x", d.GetBaseID())
	ctx, err := combineInstrsAndSign(cl, *signer, byzcoin.Instruction{
		InstanceID: gdID,
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDarcID,
			Args: byzcoin.Arguments{{
				Name:  "darc",
				Value: dBuf,
			}},
		}})
	if err != nil {
		return err
	}
	_, err = cl.AddTransactionAndWait(ctx, 5)
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write([]byte(contracts.ContractCoinID))
	h.Write(d.GetBaseID())
	coinIID := h.Sum(nil)
	log.Infof("Creating Coin for user: %x", coinIID)
	ctx, err = combineInstrsAndSign(cl, *signer, byzcoin.Instruction{
		InstanceID: gdID,
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractCoinID,
			Args: byzcoin.Arguments{{
				Name:  "type",
				Value: personhood.SpawnerCoin.Slice(),
			}, {
				Name:  "public",
				Value: d.GetBaseID(),
			}, {
				Name:  "darcID",
				Value: d.GetBaseID(),
			}},
		},
	})
	if err != nil {
		return err
	}
	_, err = cl.AddTransactionAndWait(ctx, 5)
	if err != nil {
		return err
	}

	h = sha256.New()
	h.Write([]byte(personhood.ContractCredentialID))
	h.Write(d.GetBaseID())
	credIID := h.Sum(nil)
	cred := personhood.CredentialStruct{
		Credentials: []personhood.Credential{{
			Name: "public",
			Attributes: []personhood.Attribute{{
				Name:  "ed25519",
				Value: pubBuf,
			}}},
			{
				Name: "darc",
				Attributes: []personhood.Attribute{{
					Name:  "darcID",
					Value: d.GetBaseID(),
				}}},
			{
				Name: "coin",
				Attributes: []personhood.Attribute{{
					Name:  "coinIID",
					Value: coinIID,
				}}},
		},
	}
	credBuf, err := protobuf.Encode(&cred)
	log.Infof("Creating Credentials for user: %x", credIID)
	ctx, err = combineInstrsAndSign(cl, *signer, byzcoin.Instruction{
		InstanceID: gdID,
		Spawn: &byzcoin.Spawn{
			ContractID: personhood.ContractCredentialID,
			Args: byzcoin.Arguments{{
				Name:  "darcIDBuf",
				Value: d.GetBaseID(),
			}, {
				Name:  "instID",
				Value: credIID,
			}, {
				Name:  "credential",
				Value: credBuf,
			}},
		},
	})
	if err != nil {
		return err
	}
	_, err = cl.AddTransactionAndWait(ctx, 5)
	if err != nil {
		return err
	}

	log.Info("User should be correctly registered")

	return nil
}

func show(c *cli.Context) error {
	if c.NArg() != 2 {
		return errors.New("please give the following arguments: bc-xxx.cfg credentialIID")
	}

	_, cl, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return err
	}

	credBuf, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return err
	}
	credIID := byzcoin.NewInstanceID(credBuf)

	p, err := cl.GetProofFromLatest(credIID.Slice())
	if err != nil {
		return err
	}
	if !p.Proof.InclusionProof.Match(credBuf) {
		return errors.New("this credentialIID does not exist")
	}
	val, cid, _, err := p.Proof.Get(credBuf)
	if err != nil {
		return err
	}
	if cid != personhood.ContractCredentialID {
		return errors.New("the instance at this IID is not a credential, but: " + cid)
	}
	cred, err := personhood.ContractCredentialFromBytes(val)
	if err != nil {
		return err
	}

	log.Infof("Credentials of %x", credBuf)
	for _, c := range cred.(*personhood.ContractCredential).Credentials {
		var atts []string
		for _, a := range c.Attributes {
			atts = append(atts, fmt.Sprintf("%s: %x", a.Name, a.Value))
		}
		log.Infof("\t[%s] = %s", c.Name, strings.Join(atts, "\n\t\t"))
	}
	return err
}

func combineInstrsAndSign(cl *byzcoin.Client, signer darc.Signer, instrs ...byzcoin.Instruction) (byzcoin.ClientTransaction, error) {
	gscr, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return byzcoin.ClientTransaction{}, err
	}
	for i := range instrs {
		gscr.Counters[0]++
		instrs[i].SignerCounter = gscr.Counters
	}
	ctx, err := cl.CreateTransaction(instrs...)
	if err != nil {
		return byzcoin.ClientTransaction{}, err
	}
	err = ctx.FillSignersAndSignWith(signer)
	if err != nil {
		return byzcoin.ClientTransaction{}, err
	}
	return ctx, nil
}

func verifyAdminDarc(cl *byzcoin.Client, cfg lib.Config, signer darc.Signer) error {
	gdID := cfg.AdminDarc.GetBaseID()
	p, err := cl.GetProofFromLatest(gdID)
	if err != nil {
		return err
	}
	v, _, _, err := p.Proof.Get(gdID)
	if err != nil {
		return err
	}
	gdarc, err := darc.NewFromProtobuf(v)
	if err != nil {
		return err
	}
	found := 0
	spawners := []string{"credential", "coin", "spawner"}
	invokes := []string{personhood.ContractCredentialID + ".update"}
	actions := regexp.MustCompile("(spawn:" +
		strings.Join(spawners, "|spawn:") +
		"|invoke:" + strings.Join(invokes, "|invoke:") + ")")
	for _, r := range gdarc.Rules.List {
		if actions.Match([]byte(r.Action)) {
			found++
		}
	}
	if found < len(spawners)+len(invokes) {
		log.Info("Adding spawners and invokes to genesis darc")
		gDarcNew := gdarc.Copy()
		gDarcNew.EvolveFrom(gdarc)
		for _, s := range spawners {
			r := darc.Action("spawn:" + s)
			log.Info("Adding", r)
			if !gDarcNew.Rules.Contains(r) {
				gDarcNew.Rules.AddRule(r, expression.Expr(signer.Identity().String()))
			}
		}
		for _, s := range invokes {
			r := darc.Action("invoke:" + s)
			log.Info("Adding", r)
			if !gDarcNew.Rules.Contains(r) {
				gDarcNew.Rules.AddRule(r, expression.Expr(signer.Identity().String()))
			}
		}
		darcBuf, err := gDarcNew.ToProto()
		if err != nil {
			return err
		}
		ctx, err := combineInstrsAndSign(cl, signer, byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(gdID),
			Invoke: &byzcoin.Invoke{
				Command:    "evolve",
				ContractID: byzcoin.ContractDarcID,
				Args: byzcoin.Arguments{{
					Name:  "darc",
					Value: darcBuf,
				}},
			},
		})
		log.Info("Sending evolve-instruction to byzcoin")
		_, err = cl.AddTransactionAndWait(ctx, 5)
		if err != nil {
			return err
		}
		log.Info("Successfully updated genesis-darc")
	}
	return nil
}
