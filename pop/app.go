/*
* This is a template for creating an app. It only has one command which
* prints out the name of the app.
 */
package main

import (
	"os"

	"io/ioutil"

	"path"

	"errors"

	"encoding/base64"

	"bytes"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/cosi/check"
	"github.com/dedis/cothority/pop/service"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/anon"
	"github.com/dedis/crypto/random"
	"github.com/dedis/onet/app/config"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

func init() {
	network.RegisterPacketType(Config{})
}

var client *service.Client

type Config struct {
	Private abstract.Scalar
	Public  abstract.Point
	Index   int
	Address network.Address
	Final   *service.FinalStatement
}

var mainConfig *Config
var fileConfig string

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore client"
	app.Usage = "Connects to a ssh-keystore-server and updates/changes information"
	app.Version = "0.3"
	app.Commands = []cli.Command{
		commandMgr,
		commandClient,
		{
			Name:      "check",
			Aliases:   []string{"c"},
			Usage:     "Check if the servers in the group definition are up and running",
			ArgsUsage: "group.toml",
			Action:    checkConfig,
		},
	}
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug,d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config,c",
			Value: "~/.config/cothority/pop",
			Usage: "The configuration-directory of pop",
		},
	}
	app.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		client = service.NewClient()
		fileConfig = path.Join(c.String("config"), "config.bin")
		readConfig()
		return nil
	}
	app.Run(os.Args)
}

// links this pop to a cothority
func mgrLink(c *cli.Context) error {
	log.Info("Mgr: Link")
	if c.NArg() == 0 {
		log.Fatal("Please give an IP and optionally a pin")
	}
	newConfig()
	addr := network.NewAddress(network.PlainTCP, c.Args().First())
	if err := client.Pin(addr, c.Args().Get(1), mainConfig.Public); err != nil {
		return err
	}
	mainConfig.Address = addr
	log.Info("Successfully linked with", addr)
	writeConfig()
	return nil
}

// sets up a configuration
func mgrConfig(c *cli.Context) error {
	log.Info("Mgr: Config", mainConfig.Address.String())
	if c.NArg() != 2 {
		log.Fatal("Please give pop_desc.toml and group.toml")
	}
	if mainConfig.Address.String() == "" {
		log.Fatal("No address")
		return errors.New("No address found - please link first")
	}
	desc := &service.PopDesc{}
	pdFile := c.Args().First()
	buf, err := ioutil.ReadFile(pdFile)
	log.ErrFatal(err, "While reading", pdFile)
	_, err = toml.Decode(string(buf), desc)
	log.ErrFatal(err, "While decoding", pdFile)
	group := readGroup(c.Args().Get(1))
	log.ErrFatal(check.Servers(group), "Couldn't check servers")
	desc.Roster = group.Roster
	log.ErrFatal(client.StoreConfig(mainConfig.Address, desc))
	mainConfig.Final.Desc = desc
	writeConfig()
	return nil
}

// adds a public key to the list
func mgrPublic(c *cli.Context) error {
	log.Info("Mgr: Adding public key", c.Args().First())
	if c.NArg() < 1 {
		log.Fatal("Please give a public key")
	}
	pub := service.B64ToPoint(c.Args().First())
	if pub == nil {
		log.Fatal("Couldn't parse public key")
	}
	for _, p := range mainConfig.Final.Attendees {
		if p.Equal(pub) {
			log.Fatal("This key already exists")
		}
	}
	mainConfig.Final.Attendees = append(mainConfig.Final.Attendees, pub)
	writeConfig()
	return nil
}

// finalizes the statement
func mgrFinal(c *cli.Context) error {
	log.Info("Mgr: Final")
	if len(mainConfig.Final.Attendees) == 0 {
		log.Fatal("No attendees stored - first store at least one")
	}
	if mainConfig.Address == "" {
		log.Fatal("Not linked")
	}
	if len(mainConfig.Final.Signature) > 0 {
		log.Info("Final statement already here:\n", "\n"+mainConfig.Final.ToToml())
		return nil
	}
	fs, err := client.Finalize(mainConfig.Address, mainConfig.Final.Desc, mainConfig.Final.Attendees)
	log.ErrFatal(err)
	mainConfig.Final = fs
	writeConfig()
	log.Info("Created final statement:\n", "\n"+mainConfig.Final.ToToml())
	return nil
}

// creates a new private/public pair
func clientCreate(c *cli.Context) error {
	log.Info("Client: create")
	priv := network.Suite.NewKey(random.Stream)
	pub := network.Suite.Point().Mul(nil, priv)
	log.Infof("Keypair private-public:\n%s\n%s",
		service.ScalarToB64(priv),
		service.PointToB64(pub))
	return nil
}

// joins a poparty
func clientJoin(c *cli.Context) error {
	log.Info("Client: join")
	if c.NArg() < 2 {
		log.Fatal("Please give final.toml and private key.")
	}
	finalName := c.Args().First()
	privStr := c.Args().Get(1)
	privBuf, err := base64.StdEncoding.DecodeString(privStr)
	log.ErrFatal(err)
	priv := network.Suite.Scalar()
	log.ErrFatal(priv.UnmarshalBinary(privBuf))
	buf, err := ioutil.ReadFile(finalName)
	log.ErrFatal(err)
	mainConfig.Final = service.NewFinalStatementFromString(string(buf))
	if mainConfig.Final == nil {
		log.Fatal("Couldn't parse final statement")
	}
	mainConfig.Private = priv
	mainConfig.Public = network.Suite.Point().Mul(nil, priv)
	mainConfig.Index = -1
	for i, p := range mainConfig.Final.Attendees {
		if p.Equal(mainConfig.Public) {
			log.Info("Found public key at index", i)
			mainConfig.Index = i
		}
	}
	if mainConfig.Index == -1 {
		log.Fatal("Didn't find our public key in the final statement!")
	}
	writeConfig()
	log.Info("Stored new final statement and key.")
	return nil
}

// signs a message + context
func clientSign(c *cli.Context) error {
	log.Info("Client: sign")
	if mainConfig.Index == -1 {
		log.Fatal("No public key stored.")
	}
	if c.NArg() < 2 {
		log.Fatal("Please give msg and context")
	}
	msg := []byte(c.Args().First())
	ctx := []byte(c.Args().Get(1))

	Set := anon.Set(mainConfig.Final.Attendees)
	sig := anon.Sign(network.Suite, random.Stream, msg,
		Set, ctx, mainConfig.Index, mainConfig.Private)
	tag, err := anon.Verify(network.Suite, msg, Set, ctx, sig)
	log.ErrFatal(err)
	log.Infof("\nSignature: %s\nTag: %s", base64.StdEncoding.EncodeToString(sig),
		base64.StdEncoding.EncodeToString(tag))
	return nil
}

// verifies a signature and tag
func clientVerify(c *cli.Context) error {
	log.Info("Client: verify")
	if mainConfig.Index == -1 {
		log.Fatal("No public key stored")
	}
	if c.NArg() < 4 {
		log.Fatal("Please give a msg, context, signature and a tag")
	}
	msg := []byte(c.Args().First())
	ctx := []byte(c.Args().Get(1))
	sig, err := base64.StdEncoding.DecodeString(c.Args().Get(2))
	log.ErrFatal(err)
	tag, err := base64.StdEncoding.DecodeString(c.Args().Get(3))
	log.ErrFatal(err)
	ctag, err := anon.Verify(network.Suite, msg,
		anon.Set(mainConfig.Final.Attendees), ctx, sig)
	log.ErrFatal(err)
	if !bytes.Equal(tag, ctag) {
		log.Fatalf("Tag and calculated tag are not equal:\n%x - %x", tag, ctag)
	}
	log.Info("Successfully verified signature and tag")
	return nil
}

func readGroup(name string) *config.Group {
	f, err := os.Open(name)
	log.ErrFatal(err, "Couldn't open group definition file")
	group, err := config.ReadGroupDescToml(f)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s",
			name)
	}
	return group
}

// checkConfig contacts all servers and verifies if it receives a valid
// signature from each.
func checkConfig(c *cli.Context) error {
	return check.Config(c.Args().First())
}

func newConfig() {
	mainConfig = &Config{
		Private: network.Suite.NewKey(random.Stream),
		Final: &service.FinalStatement{
			Attendees: []abstract.Point{},
			Signature: []byte{},
		},
		Index: -1,
	}
	mainConfig.Public = network.Suite.Point().Mul(nil, mainConfig.Private)
}

func readConfig() {
	file := config.TildeToHome(fileConfig)
	if _, err := os.Stat(file); err != nil {
		newConfig()
		return
	}
	buf, err := ioutil.ReadFile(file)
	if err == nil {
		_, msg, err := network.UnmarshalRegistered(buf)
		if err == nil {
			var ok bool
			mainConfig, ok = msg.(*Config)
			if ok {
				log.Lvlf2("Read config-file: %v", mainConfig)
				return
			}
		}
	}
	log.Fatal("Couldn't read", file, "- please remove it.")
}

func writeConfig() {
	buf, err := network.MarshalRegisteredType(mainConfig)
	log.ErrFatal(err)
	file := config.TildeToHome(fileConfig)
	os.MkdirAll(path.Dir(file), 0770)
	log.ErrFatal(ioutil.WriteFile(file, buf, 0660))
}
