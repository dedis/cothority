/*
* This is a template for creating an app. It only has one command which
* prints out the name of the app.
 */
package main

import (
	"os"

	"gopkg.in/dedis/onet.v1/app"

	"strings"

	"errors"

	"encoding/hex"

	"io/ioutil"

	"github.com/dedis/cothority/logread"
	"github.com/dedis/cothority/skipchain"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"gopkg.in/urfave/cli.v1"
)

type logConfig struct{}

func main() {
	network.RegisterMessage(logConfig{})
	cliApp := cli.NewApp()
	cliApp.Name = "WLogR"
	cliApp.Usage = "Write secrets to the skipchains and do logged read-requests"
	cliApp.Version = "0.1"
	cliApp.Commands = []cli.Command{
		{
			Name:    "manage",
			Usage:   "manage the wlr, acl and roles",
			Aliases: []string{"m"},
			Subcommands: []cli.Command{
				{
					Name:      "create",
					Usage:     "create a write log-read skipchain",
					Aliases:   []string{"cr"},
					ArgsUsage: "group pseudonym",
					Action:    mngCreate,
				},
				{
					Name:      "join",
					Usage:     "join a write log-read skipchain",
					Aliases:   []string{"cr"},
					ArgsUsage: "group wlr-skipchain-id private_key",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "overwrite, o",
							Usage: "overwrite existing config",
						},
					},
					Action: mngJoin,
				},
				{
					Name:    "role",
					Usage:   "role control",
					Aliases: []string{"r"},
					Subcommands: []cli.Command{
						{
							Name:      "create",
							Usage:     "create admin, writer or reader",
							Aliases:   []string{"cr"},
							ArgsUsage: "(admin|writer|reader):pseudo",
							Action:    mngRoleCreate,
						},
						{
							Name:      "add",
							Usage:     "add admin, writer or reader",
							Aliases:   []string{"a"},
							ArgsUsage: "private_key",
							Action:    mngRoleAdd,
						},
						{
							Name:      "remove",
							Usage:     "remove admin, writer or reader",
							Aliases:   []string{"rm"},
							ArgsUsage: "role:pseudo",
							Action:    mngRoleRm,
						},
						{
							Name:    "list",
							Usage:   "list all roles",
							Aliases: []string{"ls"},
							Action:  mngRoleList,
						},
					},
				},
			},
		},
		{
			Name:      "write",
			Usage:     "write to the wlr-skipchain",
			Aliases:   []string{"w"},
			ArgsUsage: "pseudonym file",
			Action:    write,
		},
		{
			Name:    "list",
			Usage:   "list all available files",
			Aliases: []string{"ls"},
			Action:  list,
		},
		{
			Name:    "read",
			Usage:   "read from the wlr-skipchain",
			Aliases: []string{"r"},
			Subcommands: []cli.Command{
				{
					Name:      "request",
					Usage:     "request a read operation",
					ArgsUsage: "pseudonym file",
					Action:    readReq,
				},
				{
					Name:      "fetch",
					Usage:     "fetch a requested file",
					ArgsUsage: "pseudonym file",
					Action:    readFetch,
				},
			},
		},
	}
	cliApp.Flags = []cli.Flag{
		app.FlagDebug,
		cli.StringFlag{
			Name:  "config, c",
			Value: "~/.config/wlogr",
			Usage: "The configuration-directory for wlogr",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	cliApp.Run(os.Args)
}

// Creates a new acl and wlr skipchain.
func mngCreate(c *cli.Context) error {
	if c.NArg() < 2 {
		log.Fatal("Please give group-toml and pseudo")
	}
	log.Info("Creating ACL- and WLR-skipchain")
	pseudo := c.Args().Get(1)
	group := getGroup(c)
	cl := logread.NewClient()
	acl, wlr, admin, err := cl.CreateSkipchains(group.Roster, pseudo)
	log.ErrFatal(err)
	cfg := &wlrConfig{
		ACLBunch: skipchain.NewSkipBlockBunch(acl),
		WLRBunch: skipchain.NewSkipBlockBunch(wlr),
		Roles:    logread.NewCredentials(admin),
	}
	log.Infof("Created new skipchains and added %s as admin", pseudo)
	log.Infof("WLR-skipchainid: %x", cfg.WLRBunch.GenesisID)
	return cfg.saveConfig(c)
}

// Joins an existing wlr skipchain.
func mngJoin(c *cli.Context) error {
	if c.NArg() < 3 {
		log.Fatal("Please give: group wlr-skipchain-id private_key")
	}
	cfg, loaded := loadConfig(c)
	if loaded && !c.Bool("overwrite") {
		log.Fatal("Config is present but overwrite-flag not given")
	}
	cfg = &wlrConfig{}
	group := getGroup(c)
	sid, err := hex.DecodeString(c.Args().Get(1))
	log.ErrFatal(err)
	private, err := crypto.StringHexToScalar(network.Suite, c.Args().Get(2))
	log.ErrFatal(err)
	public := network.Suite.Point().Mul(nil, private)
	log.Info("Public key is:", public.String())
	log.Info("Joining ACL-skipchain")
	cfg.WLRBunch, err = CreateWLRBunch(group.Roster, sid)
	log.ErrFatal(err)
	wlrGenesis := cfg.WLRBunch.GetByID(cfg.WLRBunch.GenesisID)
	wlrData := logread.NewDataWlr(wlrGenesis.Data)
	cfg.ACLBunch, err = CreateACLBunch(group.Roster, wlrData.Config.ACL)
	log.ErrFatal(err)
	acl := cfg.Acls()
	if acl == nil {
		log.Fatal("Empty data in ACL-skipchain")
	}
	var cr *logread.Credential
	if cr = acl.Admins.SearchPublic(public); cr != nil {
		log.Info("Found admin user", cr.Pseudonym)
	} else if cr = acl.Writers.SearchPublic(public); cr != nil {
		log.Info("Found writer", cr.Pseudonym)
	} else if cr = acl.Readers.SearchPublic(public); cr != nil {
		log.Info("Found reader", cr.Pseudonym)
	} else {
		log.Info(acl)
		return errors.New("credential not found")
	}
	cr.Private = private
	cfg.Roles = logread.NewCredentials(cr)
	log.Print(cfg.Roles)
	return cfg.saveConfig(c)
}

func mngRoleCreate(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	admin := cfg.Admin()
	if admin == nil {
		log.Fatal("You don't have an admin-role")
	}
	if c.NArg() < 1 {
		log.Fatal("Please give role:pseudo")
	}
	rolePseudo := strings.Split(c.Args().First(), ":")
	if len(rolePseudo) != 2 {
		log.Fatal("Please give role:pseudo")
	}
	role, pseudo := rolePseudo[0], rolePseudo[1]
	acls := cfg.Acls()
	var cred *logread.Credential
	switch strings.ToLower(role) {
	case "admin":
		if acls.Admins.SearchPseudo(pseudo) != nil {
			log.Fatal("Pseudo already exists")
		}
		cred = acls.Admins.AddPseudo(pseudo)
	case "writer":
		if acls.Writers.SearchPseudo(pseudo) != nil {
			log.Fatal("Pseudo already exists")
		}
		cred = acls.Writers.AddPseudo(pseudo)
	case "reader":
		if acls.Readers.SearchPseudo(pseudo) != nil {
			log.Fatal("Pseudo already exists")
		}
		cred = acls.Readers.AddPseudo(pseudo)
	default:
		return errors.New("Didn't find role")
	}
	log.Infof("Added role:%s for pseudo:%s", role, pseudo)
	cfg.Roles.List = append(cfg.Roles.List, cred)
	priv, err := cred.Private.MarshalBinary()
	log.ErrFatal(err)
	log.Infof("Private key: %x", priv)

	_, err = logread.NewClient().EvolveACL(cfg.ACLBunch, acls, admin)
	if err != nil {
		return err
	}
	log.Info(cfg.Acls())
	return cfg.saveConfig(c)
}
func mngRoleAdd(c *cli.Context) error {
	log.Info("")
	return nil
}
func mngRoleRm(c *cli.Context) error {
	log.Info("")
	return nil
}
func mngRoleList(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	log.Info("Current ACLs:\n", cfg.Acls())
	return nil
}

func write(c *cli.Context) error {
	if c.NArg() < 2 {
		log.Fatal("Please give the following: writer file")
	}
	cfg := loadConfigOrFail(c)
	writer, file := c.Args().Get(0), c.Args().Get(1)
	log.Info("Going to write file to skipchain under writer", writer)
	sb, err := cfg.StoreFile(writer, file)
	if err != nil {
		return err
	}
	log.Infof("Stored file %s in skipblock %x", file, sb.Hash)
	return nil
}

func list(c *cli.Context) error {
	log.Info("Listing existing files")
	cfg := loadConfigOrFail(c)
	for _, sb := range cfg.WLRBunch.SkipBlocks {
		log.Info(logread.NewDataWlr(sb.Data))
	}
	return nil
}

func readReq(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	if c.NArg() < 2 {
		log.Fatal("Please give the following: pseudo file-id")
	}
	pseudo := c.Args().Get(0)
	cred := cfg.Roles.SearchPseudo(pseudo)
	if cred == nil || cred.Private == nil {
		log.Fatal("Don't have credentials for reader", pseudo)
	}
	if cfg.Acls().Readers.SearchPseudo(pseudo) == nil {
		log.Fatal("Reader", pseudo, "not in acl-read")
	}
	file, err := hex.DecodeString(c.Args().Get(1))
	log.ErrFatal(err)
	log.Infof("Requesting read-access to file %x", file)
	sb, cerr := logread.NewClient().ReadRequest(cfg.WLRBunch.Latest, cred, file)
	log.ErrFatal(cerr)
	if sb == nil {
		log.Fatal("Got empty skipblock")
	}
	_, dwI, err := network.Unmarshal(sb.Data)
	dw, ok := dwI.(*logread.DataWlr)
	if !ok {
		log.Fatal("Didn't find read-request")
	}
	req := dw.Read
	if req.Pseudonym != pseudo {
		log.Fatal("Got wrong pseudo")
	}
	if !req.File.Equal(file) {
		log.Fatal("Got wrong file")
	}
	if crypto.VerifySchnorr(network.Suite, cred.Public, req.File, *req.Signature) != nil {
		log.Fatal("Wrong signature")
	}
	log.Info("Successfully added read-request to skipchain.")
	log.Infof("Request-id is: %x", sb.Hash)
	cfg.WLRBunch.Store(sb)
	return cfg.saveConfig(c)
}
func readFetch(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	if c.NArg() < 2 {
		log.Fatal("Please give the following: read-request-id filename")
	}
	read, err := hex.DecodeString(c.Args().First())
	log.ErrFatal(err)
	file := c.Args().Get(1)
	log.Info("Writing to file:", file)
	sb := cfg.WLRBunch.GetByID(read)
	if sb == nil {
		log.Fatal("Didn't find read-request-id")
	}
	dwlr := logread.NewDataWlr(sb.Data)
	if dwlr == nil || dwlr.Read == nil {
		log.Fatal("This is not a read-request-id")
	}
	key, cerr := logread.NewClient().DecryptKeyRequest(sb, sb.Roster)
	log.ErrFatal(cerr)
	sbs, cerr := skipchain.NewClient().GetBlocks(sb.Roster, nil, dwlr.Read.File, 0)
	log.ErrFatal(cerr)
	dwlrFile := logread.NewDataWlr(sbs[0].Data)
	if dwlrFile == nil || dwlrFile.Write == nil {
		log.Fatal("Referenced file does not exist")
	}
	dataEnc := dwlrFile.Write.File
	log.Printf("Using key %x", key)
	cipher := network.Suite.Cipher(key)
	data, err := cipher.Open(nil, dataEnc)
	log.ErrFatal(err)
	ioutil.WriteFile(file, data, 0640)
	log.Infof("Successfully written %d bytes to %s", len(data), file)
	return nil
}
