/*
* This is a template for creating an app. It only has one command which
* prints out the name of the app.
 */
package main

import (
	"os"

	"gopkg.in/dedis/onet.v1/app"

	"encoding/hex"

	"io/ioutil"

	"fmt"

	"errors"

	"github.com/dedis/onchain-secrets"
	"gopkg.in/dedis/cothority.v1/skipchain"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"gopkg.in/urfave/cli.v1"
)

type logConfig struct{}

func main() {
	network.RegisterMessage(logConfig{})
	cliApp := cli.NewApp()
	cliApp.Name = "OCS"
	cliApp.Usage = "Write secrets to the skipchains and do logged read-requests"
	cliApp.Version = "0.1"
	cliApp.Commands = []cli.Command{
		{
			Name:    "manage",
			Usage:   "manage the doc, acl and roles",
			Aliases: []string{"m"},
			Subcommands: []cli.Command{
				{
					Name:      "create",
					Usage:     "create a write log-read skipchain",
					Aliases:   []string{"cr"},
					ArgsUsage: "group [private_key]",
					Action:    mngCreate,
				},
				{
					Name:    "list",
					Usage:   "list the id of the log-read skipchain",
					Aliases: []string{"ls"},
					Action:  mngList,
				},
				{
					Name:      "join",
					Usage:     "join a write log-read skipchain",
					Aliases:   []string{"j"},
					ArgsUsage: "group ocs-skipchain-id [private_key]",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "force, f",
							Usage: "overwrite existing config",
						},
					},
					Action: mngJoin,
				},
			},
		},
		{
			Name:    "keypair",
			Usage:   "create a keypair and write it to stdout",
			Aliases: []string{"kp"},
			Action:  keypair,
		},
		{
			Name:      "write",
			Usage:     "write to the ocs-skipchain",
			Aliases:   []string{"w"},
			ArgsUsage: "file",
			Action:    write,
		},
		{
			Name:    "list",
			Usage:   "list all available files",
			Aliases: []string{"ls"},
			Action:  list,
		},
		{
			Name:      "read",
			Usage:     "read from the ocs-skipchain",
			Aliases:   []string{"r"},
			ArgsUsage: "file_id private_key",
			Action:    read,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "o, output",
					Usage: "output file",
				},
			},
		},
		{
			Name:      "skipchain",
			Usage:     "read a block from the skipchain",
			Aliases:   []string{"sc"},
			ArgsUsage: "[skipblockID]",
			Action:    scread,
		},
	}
	cliApp.Flags = []cli.Flag{
		app.FlagDebug,
		cli.StringFlag{
			Name:  "config, c",
			Value: "~/.config/ocs",
			Usage: "The configuration-directory for ocs",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	cliApp.Run(os.Args)
}

// Creates a new ocs skipchain.
func mngCreate(c *cli.Context) error {
	if c.NArg() < 1 {
		log.Fatal("Please give group-toml [pseudo]")
	}
	log.Info("Creating OCS-skipchain")
	pseudo := c.Args().Get(1)
	group := getGroup(c)
	cl := ocs.NewClient()
	scurl, err := cl.CreateSkipchain(group.Roster)
	log.ErrFatal(err)
	cfg := &ocsConfig{
		SkipChainURL: scurl,
	}
	log.Infof("Created new skipchains and added %s as admin", pseudo)
	log.Infof("OCS-skipchainid: %x", scurl.Genesis)
	return cfg.saveConfig(c)
}

// Prints the id of the OCS-skipchain
func mngList(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	log.Infof("OCS-skipchainid:\t%x", cfg.SkipChainURL.Genesis)
	return nil
}

// Joins an existing OCS skipchain.
func mngJoin(c *cli.Context) error {
	if c.NArg() < 2 {
		log.Fatal("Please give: group ocs-skipchain-id")
	}
	cfg, loaded := loadConfig(c)
	if loaded && !c.Bool("force") {
		log.Fatal("Config is present but overwrite-flag not given")
	}
	cfg = &ocsConfig{}
	group := getGroup(c)
	sid, err := hex.DecodeString(c.Args().Get(1))
	log.ErrFatal(err)
	cfg.SkipChainURL = &ocs.SkipChainURL{
		Roster:  group.Roster,
		Genesis: sid,
	}
	log.ErrFatal(err)
	return cfg.saveConfig(c)
}

func keypair(c *cli.Context) error {
	//kp := config.NewKeyPair(network.Suite)
	r, err := crypto.StringHexToScalar(network.Suite, "5046ADC1DBA838867B2BBBFDD0C3423E58B57970B5267A90F57960924A87F156")
	privStr, err := crypto.ScalarToStringHex(network.Suite, r)
	//privStr, err := crypto.ScalarToString64(network.Suite, kp.Secret)
	if err != nil {
		return err
	}
	log.ErrFatal(err)
	pub := network.Suite.Point().Mul(nil, r)
	pubStr, err := crypto.PubToStringHex(network.Suite, pub)
	//pubStr, err := crypto.PubToString64(network.Suite, kp.Public)
	if err != nil {
		return err
	}
	fmt.Printf("%s:%s\n", privStr, pubStr)
	return nil
}

func write(c *cli.Context) error {
	if c.NArg() < 2 {
		log.Fatal("Please give the following: file reader1[,reader2,...]")
	}
	cfg := loadConfigOrFail(c)
	file := c.Args().Get(0)

	log.Info("Going to write file to skipchain")
	data, err := ioutil.ReadFile(file)
	log.ErrFatal(err)
	//symKey := random.Bytes(32, random.Stream)
	symKey, err := hex.DecodeString("294AEDA9694E0391EEC2D8C133BEBBFF")
	log.ErrFatal(err)
	cipher := network.Suite.Cipher(symKey)
	encData := cipher.Seal(nil, data)
	darc := ocs.NewDarc(cfg.SkipChainURL.Genesis)
	darc.Public = []abstract.Point{}
	for _, r := range c.Args().Tail() {
		pub, err := crypto.StringHexToPub(network.Suite, r)
		log.ErrFatal(err)
		darc.Public = append(darc.Public, pub)
	}

	sb, err := ocs.NewClient().WriteRequest(cfg.SkipChainURL, encData, symKey, darc)
	log.ErrFatal(err)
	log.Infof("Stored file %s in skipblock:\t%x", file, sb.Hash)
	return nil
}

func list(c *cli.Context) error {
	log.Info("Listing existing files - not possible for the moment")
	return nil
}

func read(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	if c.NArg() < 2 {
		log.Fatal("Please give the following: fileID private_key")
	}
	fileID, err := hex.DecodeString(c.Args().Get(0))
	log.ErrFatal(err)
	log.Infof("Requesting read-access to file %x", fileID)
	priv, err := crypto.StringHexToScalar(network.Suite, c.Args().Get(1))
	log.ErrFatal(err)
	pub := network.Suite.Point().Mul(nil, priv)
	log.Printf("Private: %s\nPublic: %s", priv, pub)
	cl := ocs.NewClient()
	sb, cerr := cl.ReadRequest(cfg.SkipChainURL, fileID, priv)
	log.ErrFatal(cerr)
	if sb == nil {
		log.Fatal("Got empty skipblock - read refused")
	}

	log.Info("Asking to re-encrypt the symmetric key")
	key, cerr := cl.DecryptKeyRequest(cfg.SkipChainURL, sb.Hash, priv)
	log.ErrFatal(cerr)
	fileEnc, cerr := cl.GetData(cfg.SkipChainURL, fileID)
	log.ErrFatal(cerr)
	cipher := network.Suite.Cipher(key)
	file, err := cipher.Open(nil, fileEnc)
	log.ErrFatal(err)

	log.Info("Outputting file")
	if out := c.String("o"); out != "" {
		return ioutil.WriteFile(out, file, 0660)
	}
	fmt.Println(file)
	return nil
}

func scread(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	var sc skipchain.SkipBlockID
	if c.NArg() >= 1 {
		var err error
		sc, err = hex.DecodeString(c.Args().First())
		if err != nil {
			return err
		}
	} else {
		sc = cfg.SkipChainURL.Genesis
	}
	cl := skipchain.NewClient()
	sb, cerr := cl.GetSingleBlock(cfg.SkipChainURL.Roster, sc)
	if cerr != nil {
		return cerr
	}
	log.Printf("SkipblockID (Hash): %x", sb.Hash)
	log.Printf("Index: %d", sb.Index)
	ocs := ocs.NewDataOCS(sb.Data)
	if ocs == nil {
		return errors.New("wrong data in skipblock")
	}
	if ocs.Write != nil {
		log.Printf("Writer: %#v", ocs.Write)
	}
	if ocs.Read != nil {
		log.Printf("Read: %#v", ocs.Read)
	}
	if ocs.Readers != nil {
		log.Printf("Readers: %#v", ocs.Readers)
	}
	if len(sb.ForwardLink) > 0 {
		log.Printf("Next block: %x", sb.ForwardLink[0].Hash)
	} else {
		log.Print("This is the last block")
	}
	return nil
}
