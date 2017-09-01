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

	"github.com/dedis/onchain-secrets"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
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
					ArgsUsage: "group doc-skipchain-id [private_key]",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "overwrite, o",
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
			Usage:     "write to the doc-skipchain",
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
			Usage:     "read from the doc-skipchain",
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

// Creates a new acl and doc skipchain.
func mngCreate(c *cli.Context) error {
	if c.NArg() < 1 {
		log.Fatal("Please give group-toml [pseudo]")
	}
	log.Info("Creating ACL- and Doc-skipchain")
	pseudo := c.Args().Get(1)
	group := getGroup(c)
	cl := ocs.NewClient()
	sc, err := cl.CreateSkipchain(group.Roster)
	log.ErrFatal(err)
	cfg := &ocsConfig{
		Bunch: ocs.NewSkipBlockBunch(sc),
	}
	log.Infof("Created new skipchains and added %s as admin", pseudo)
	log.Infof("Doc-skipchainid: %x", cfg.Bunch.GenesisID)
	return cfg.saveConfig(c)
}

// Prints the id of the Doc-skipchain
func mngList(c *cli.Context) error {
	cfg := loadConfigOrFail(c)
	log.Infof("OCS-skipchainid:\t%x", cfg.Bunch.GenesisID)
	return nil
}

// Joins an existing doc skipchain.
func mngJoin(c *cli.Context) error {
	if c.NArg() < 2 {
		log.Fatal("Please give: group doc-skipchain-id")
	}
	cfg, loaded := loadConfig(c)
	if loaded && !c.Bool("overwrite") {
		log.Fatal("Config is present but overwrite-flag not given")
	}
	cfg = &ocsConfig{}
	group := getGroup(c)
	sid, err := hex.DecodeString(c.Args().Get(1))
	log.ErrFatal(err)
	cfg.Bunch, err = CreateBunch(group.Roster, sid)
	log.ErrFatal(err)
	return cfg.saveConfig(c)
}

func keypair(c *cli.Context) error {
	kp := config.NewKeyPair(network.Suite)
	privStr, err := crypto.ScalarToString64(network.Suite, kp.Secret)
	if err != nil {
		return err
	}
	pubStr, err := crypto.PubToString64(network.Suite, kp.Public)
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
	var readers []abstract.Point
	for _, r := range c.Args().Tail() {
		pub, err := crypto.String64ToPub(network.Suite, r)
		log.ErrFatal(err)
		readers = append(readers, pub)
	}

	log.Info("Going to write file to skipchain")
	data, err := ioutil.ReadFile(file)
	log.ErrFatal(err)
	symKey := random.Bytes(32, random.Stream)
	cipher := network.Suite.Cipher(symKey)
	encData := cipher.Seal(nil, data)

	sb, err := ocs.NewClient().WriteRequest(cfg.Bunch.Latest, encData, symKey, readers)
	log.ErrFatal(err)
	log.Infof("Stored file %s in skipblock:\t%x", file, sb.Hash)
	return nil
}

func list(c *cli.Context) error {
	log.Info("Listing existing files")
	cfg := loadConfigOrFail(c)
	for _, sb := range cfg.Bunch.SkipBlocks {
		log.Info(ocs.NewDataOCS(sb.Data))
	}
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
	priv, err := crypto.String64ToScalar(network.Suite, c.Args().Get(1))
	log.ErrFatal(err)
	cl := ocs.NewClient()
	sb, cerr := cl.ReadRequest(cfg.Bunch.Latest, priv, fileID)
	log.ErrFatal(cerr)
	if sb == nil {
		log.Fatal("Got empty skipblock - read refused")
	}
	cfg.Bunch.Store(sb)

	key, cerr := cl.DecryptKeyRequest(sb, priv)
	log.ErrFatal(cerr)
	fileEnc, cerr := cl.GetFile(sb.Roster, fileID)
	log.ErrFatal(cerr)
	cipher := network.Suite.Cipher(key)
	file, err := cipher.Open(nil, fileEnc)
	log.ErrFatal(err)

	if out := c.String("o"); out != "" {
		return ioutil.WriteFile(out, file, 0660)
	}
	fmt.Println(file)
	return nil
}
