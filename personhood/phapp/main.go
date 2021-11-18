package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go.dedis.ch/cothority/v3/personhood/user"
	"net"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strings"

	"go.dedis.ch/cothority/v3/personhood/contracts"

	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	byz_contracts "go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/cothority/v3/personhood"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
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
		Name:  "adminDarcIDs",
		Usage: "get and set admin darc IDs",
		Subcommands: cli.Commands{
			{
				Name:      "get",
				Usage:     "show admin darc IDs",
				Action:    adminDarcIDsGet,
				ArgsUsage: "bc-xxx.cfg",
			},
			{
				Name:      "set",
				Usage:     "set admin darc IDs",
				Action:    adminDarcIDsSet,
				ArgsUsage: "bc-xxx.cfg key-xxx.cfg id1 id2 ...",
			},
		},
	},
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
	{
		Name:      "user",
		Usage:     "create a new user",
		ArgsUsage: "bc-xxx.cfg key-xxx.cfg baseURL alias",
		Action:    createUser,
	},
	{
		Name:  "email",
		Usage: "use the email service",
		Subcommands: cli.Commands{
			{
				Name:   "signup",
				Usage:  "signup a new user to the email service",
				Action: emailSignup,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "addr",
						Usage: "the address of the remote node",
					},
					cli.StringFlag{
						Name:  "alias",
						Usage: "alias of the new user",
					},
					cli.StringFlag{
						Name:  "email",
						Usage: "email address of the new user",
					},
				},
			},
			{
				Name:   "recover",
				Usage:  "recover an existing user",
				Action: emailRecovery,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "addr",
						Usage: "the address of the remote node",
					},
					cli.StringFlag{
						Name:  "email",
						Usage: "email address of the user to be recovered",
					},
				},
			},
			{
				Name:  "setup",
				Usage: "setup the email service",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:     "bcID",
						Usage:    "ByzCoin ID",
						Required: true,
					},
					cli.StringFlag{
						Name:     "private",
						Usage:    "private.toml of node to communicate",
						Required: true,
					},
					cli.StringFlag{
						Name: "user_device",
						Usage: "signup string for the user that can spawn" +
							" other users",
						Required: true,
					},
					cli.StringFlag{
						Name: "baseURL",
						Usage: "url to be used when creating signup or" +
							" recovery URLs",
						Required: false,
					},
					cli.StringFlag{
						Name: "darcID",
						Usage: "the instance-ID of the DARC where new users" +
							" will be added to",
						Required: false,
					},
					cli.StringFlag{
						Name:     "smtp_host",
						Usage:    "Host:port where the SMTP server can be reached",
						Required: true,
					},
					cli.StringFlag{
						Name: "smtp_from",
						Usage: "email address in the FROM field that allows" +
							" to send emails without password",
						Required: true,
					},
					cli.StringFlag{
						Name:     "smtp_reply_to",
						Usage:    "email address for the REPLY_TO field",
						Required: true,
					},
				},
				Action: emailSetup,
			},
		},
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
	err := cliApp.Run(os.Args)
	if err != nil {
		log.Fatalf("Error while running app: %+v", err)
	}
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
			Name:  contracts.SpawnerCoin,
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
			ContractID: contracts.ContractCredentialID,
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
			Name:  contracts.SpawnerCoin,
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
			ContractID: contracts.ContractSpawnerID,
			Args:       args,
		},
	})
	if err != nil {
		return err
	}
	log.Infof("%+v", ctx.Instructions[0])
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
	for _, s := range []string{contracts.ContractCredentialID + ".update", byz_contracts.ContractCoinID + ".fetch", byz_contracts.ContractCoinID + ".transfer"} {
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
	h.Write([]byte(byz_contracts.ContractCoinID))
	h.Write(d.GetBaseID())
	coinIID := h.Sum(nil)
	log.Infof("Creating Coin for user: %x", coinIID)
	ctx, err = combineInstrsAndSign(cl, *signer, byzcoin.Instruction{
		InstanceID: gdID,
		Spawn: &byzcoin.Spawn{
			ContractID: byz_contracts.ContractCoinID,
			Args: byzcoin.Arguments{{
				Name:  "type",
				Value: contracts.SpawnerCoin.Slice(),
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
	h.Write([]byte(contracts.ContractCredentialID))
	h.Write(d.GetBaseID())
	credIID := h.Sum(nil)
	cred := contracts.CredentialStruct{
		Credentials: []contracts.Credential{{
			Name: "public",
			Attributes: []contracts.Attribute{{
				Name:  "ed25519",
				Value: pubBuf,
			}}},
			{
				Name: "darc",
				Attributes: []contracts.Attribute{{
					Name:  "darcID",
					Value: d.GetBaseID(),
				}}},
			{
				Name: "coin",
				Attributes: []contracts.Attribute{{
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
			ContractID: contracts.ContractCredentialID,
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
	if cid != contracts.ContractCredentialID {
		return errors.New("the instance at this IID is not a credential, but: " + cid)
	}
	cred, err := contracts.ContractCredentialFromBytes(val)
	if err != nil {
		return err
	}

	log.Infof("Credentials of %x", credBuf)
	for _, c := range cred.(*contracts.ContractCredential).Credentials {
		var atts []string
		for _, a := range c.Attributes {
			atts = append(atts, fmt.Sprintf("%s: %x", a.Name, a.Value))
		}
		log.Infof("\t[%s] = %s", c.Name, strings.Join(atts, "\n\t\t"))
	}
	return err
}

func adminDarcIDsGet(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("please give the following argument: public.toml")
	}

	pt, err := os.Open(c.Args().First())
	if err != nil {
		return xerrors.Errorf("couldn't open file: %v", err)
	}
	group, err := app.ReadGroupDescToml(pt)
	if err != nil {
		return xerrors.Errorf("wrong public.toml: %v", err)
	}
	if len(group.Roster.List) == 0 {
		return xerrors.New("no server defined")
	}

	cl := personhood.NewClient()
	log.Info("Fetching all admin-darc-IDs")
	rep, errs := cl.GetAdminDarcIDs(group.Roster.List[0])
	if len(errs) > 0 {
		return xerrors.Errorf("got error while fetching ids: %s", errs)
	}
	for i, id := range rep.AdminDarcIDs {
		log.Infof("Admin darc ID #%d: %x", i, id[:])
	}

	return nil
}

func adminDarcIDsSet(c *cli.Context) error {
	if c.NArg() == 0 {
		return errors.New("please give the following arguments: private.toml" +
			" [id1 [id2...]]")
	}

	ccfg, err := app.LoadCothority(c.Args().First())
	if err != nil {
		return err
	}
	si, err := ccfg.GetServerIdentity()
	if err != nil {
		return err
	}

	dids := make([]darc.ID, c.NArg()-1)
	for i, idStr := range c.Args()[1:] {
		did, err := hex.DecodeString(idStr)
		if err != nil {
			return xerrors.Errorf("couldn't parse id %d: %v", i, err)
		}
		dids[i] = did
	}

	cl := personhood.NewClient()
	errs := cl.SetAdminDarcIDs(si, dids, si.GetPrivate())
	if len(errs) > 0 {
		return xerrors.Errorf("couldn't set admin darc IDs: %+v", errs)
	}
	log.Info("Successfully set admin darc IDs")

	return nil
}

func emailSetup(c *cli.Context) error {
	si, es, err := emailGetRequest(c)
	if err != nil {
		return xerrors.Errorf("while parsing arguments: %v", err)
	}
	log.Info("Sending to host", si.Address)
	if err := personhood.NewClient().EmailSetup(si, es); err != nil {
		return xerrors.Errorf("couldn't send to node: %v", err)
	}
	log.Info("Successfully set up host")
	return nil
}

func emailSignup(c *cli.Context) error {
	si, alias, email, err := emailGetAddrEmail(c)
	if err != nil {
		return xerrors.Errorf("while parsing arguments: %v", err)
	}

	log.Infof("Asking %s to sign up new user %s", si.Address, email)

	reply, err := personhood.NewClient().EmailSignup(si, alias,
		email)
	if err != nil {
		return xerrors.Errorf("signup of new account failed: %v", err)
	}

	switch reply.Status {
	case personhood.ESECreated:
		log.Info("Account created successfully. The email has been sent to", email)
	case personhood.ESEExists:
		return xerrors.New("This email address already exists")
	case personhood.ESETooManyRequests:
		return xerrors.New("Exceeded quota of today's emails")
	}
	return nil
}

func emailRecovery(c *cli.Context) error {
	si, _, email, err := emailGetAddrEmail(c)
	if err != nil {
		return xerrors.Errorf("while parsing arguments: %v", err)
	}

	log.Infof("Asking %s to recover existing user %s", si.Address, email)

	reply, err := personhood.NewClient().EmailRecover(si, email)
	if err != nil {
		return xerrors.Errorf("recovery of account failed: %v", err)
	}

	switch reply.Status {
	case personhood.ERERecovered:
		log.Info("Account recovered successfully. The email has been sent to",
			email)
	case personhood.EREUnknown:
		return xerrors.New("This email address does not already exist")
	case personhood.ERETooManyRequests:
		return xerrors.New("Exceeded quota of today's emails")
	}
	return nil
}

func emailGetAddrEmail(c *cli.Context) (*network.ServerIdentity, string,
	string, error) {
	if c.NArg() != 3 {
		return nil, "", "", xerrors.Errorf("Please give the following arguments:" +
			"host:port alias email-address")
	}
	hp := c.String("node")
	if _, err := url.Parse(hp); err != nil {
		return nil, "", "", xerrors.Errorf("couldn't parse node address: %v", err)
	}
	si := &network.ServerIdentity{
		Address: network.Address(hp),
	}
	email := c.String("email")
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, "", "", xerrors.Errorf("couldn't parse email address: %v", err)
	}

	return si, c.String("alias"), email, nil
}

func emailGetRequest(c *cli.Context) (si *network.ServerIdentity,
	es *personhood.EmailSetup,
	err error) {
	es = &personhood.EmailSetup{}

	bcIDStr := c.String("bcID")
	es.ByzCoinID, err = hex.DecodeString(bcIDStr)
	if err != nil || len(es.ByzCoinID) != 32 {
		return nil, nil, xerrors.New("ByzCoinID needs to be an InstanceID of length 32" +
			" bytes, encoded in hexadecimal")
	}
	privateToml := c.String("private")
	if _, err := os.Stat(privateToml); os.IsNotExist(err) {
		return nil, nil, xerrors.New("private.toml file doesn't exist")
	}
	ccfg, err := app.LoadCothority(privateToml)
	if err != nil {
		return nil, nil, xerrors.Errorf("couldn't load private.toml: %v", err)
	}
	si, err = ccfg.GetServerIdentity()
	if err != nil {
		return nil, nil, xerrors.Errorf("while getting server identity: %v", err)
	}
	es.DeviceURL = c.String("user_device")
	// Start with baseURL as derived from the deviceURL
	baseURL, err := url.Parse(es.DeviceURL)
	if err != nil {
		return nil, nil, xerrors.Errorf(
			"user_device needs to be a valid URL: %s", es.DeviceURL)
	}
	if es.BaseURL = c.String("baseURL"); es.BaseURL != "" {
		baseURL, err = url.Parse(es.BaseURL)
		if err != nil {
			return nil, nil, xerrors.New("baseURL needs to be a valid URL")
		}
	}
	es.BaseURL = fmt.Sprintf("%s://%s", baseURL.Scheme, baseURL.Host)
	if baseURL.Path != "" {
		es.BaseURL = es.BaseURL + baseURL.Path
	}

	darcIDStr := c.String("darcID")
	edID, err := hex.DecodeString(darcIDStr)
	if err != nil || len(edID) != 32 {
		log.Warn("Didn't get darcID - will create a new darc on the user")
		es.EmailDarcID = byzcoin.NewInstanceID([]byte{})
	} else {
		es.EmailDarcID = byzcoin.NewInstanceID(edID)
	}
	es.SMTPHost = c.String("smtp_host")
	if _, _, err := net.SplitHostPort(es.SMTPHost); err != nil {
		return nil, nil, xerrors.New("smtp_host needs to be a valid host:port")
	}
	es.SMTPFrom = c.String("smtp_from")
	if _, err := mail.ParseAddress(es.SMTPFrom); err != nil {
		return nil, nil, xerrors.New("smtp_from needs to be a valid email address")
	}
	es.SMTPReplyTo = c.String("smtp_reply_to")
	if _, err := mail.ParseAddress(es.SMTPReplyTo); err != nil {
		return nil, nil, xerrors.New("smtp_reply_to needs to be a valid email address")
	}

	return
}

func createUser(c *cli.Context) error {
	if c.NArg() != 4 {
		return errors.New("please give the following arguments: " +
			"bc-xxx.cfg key-xxx.cfg baseURL alias")
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
	baseURL := c.Args().Get(2)
	alias := c.Args().Get(3)

	log.Info("Creating new user from darc...")

	newUser, err := user.NewFromByzcoin(cl, cfg.AdminDarc.GetBaseID(),
		*signer, alias)
	if err != nil {
		return xerrors.Errorf("couldn't create user: %v", err)
	}
	userURL, err := newUser.CreateLink(baseURL)
	if err != nil {
		return xerrors.Errorf("couldn't create user link: %v", err)
	}

	log.Info("Created new user - signup-URL is:", userURL)
	return nil
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
	var gdarc darc.Darc
	if _, err := cl.GetInstance(byzcoin.NewInstanceID(gdID),
		byzcoin.ContractDarcID, &gdarc); err != nil {
		return xerrors.Errorf("couldn't get admin darc: %v", err)
	}

	found := 0
	spawners := []string{"credential", "coin", "spawner"}
	invokes := []string{contracts.ContractCredentialID + ".update"}
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
		gDarcNew.EvolveFrom(&gdarc)
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
