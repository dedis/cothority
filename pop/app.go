package main

import (
	"encoding/base64"
	"errors"
	"os"
	"path"

	"github.com/dedis/cothority/cosi/check"
	_ "github.com/dedis/cothority/cosi/protocol"
	_ "github.com/dedis/cothority/cosi/service"

	"fmt"
	"io/ioutil"

	"net"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/pop/service"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1/app"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"gopkg.in/urfave/cli.v1"
)

func init() {
	network.RegisterMessage(Config{})
}

// Config represents either a manager or an attendee configuration.
type Config struct {
	// Index of the attendee in the final statement. If the index
	// is -1, then this pop holds an organizer.
	Index int
	// Private key of attendee or organizer, depending on value
	// of Index.
	Private abstract.Scalar
	// Public key of attendee or organizer, depending on value of
	// index.
	Public abstract.Point
	// Address of the linked conode.
	Address network.Address
	// Final statement of the party.
	Final *service.FinalStatement
	// config-file name
	name string
}

func main() {
	appCli := cli.NewApp()
	appCli.Name = "Proof-of-personhood party"
	appCli.Usage = "Handles party-creation, finalizing, pop-token creation, and verification"
	appCli.Version = "0.1"
	appCli.Commands = []cli.Command{}
	appCli.Commands = []cli.Command{
		commandOrg,
		commandClient,
		{
			Name:      "check",
			Aliases:   []string{"c"},
			Usage:     "Check if the servers in the group definition are up and running",
			ArgsUsage: "group.toml",
			Action: func(c *cli.Context) error {
				return check.Config(c.Args().First(), false)
			},
		},
	}
	appCli.Flags = []cli.Flag{
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
	appCli.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	appCli.Run(os.Args)
}

// links this pop to a cothority
func orgLink(c *cli.Context) error {
	log.Info("Org: Link")
	if c.NArg() == 0 {
		log.Fatal("Please give an IP and optionally a pin")
	}
	cfg, client := getConfigClient(c)

	host, port, err := net.SplitHostPort(c.Args().First())
	if err != nil {
		return err
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return err
	}
	addr := network.NewTCPAddress(fmt.Sprintf("%s:%s", addrs[0], port))
	pin := c.Args().Get(1)
	if err := client.PinRequest(addr, pin, cfg.Public); err != nil {
		if err.ErrorCode() == service.ErrorWrongPIN && pin == "" {
			log.Info("Please read PIN in server-log")
			return nil
		}
		return err
	}
	cfg.Address = addr
	log.Info("Successfully linked with", addr)
	cfg.write()
	return nil
}

// sets up a configuration
func orgConfig(c *cli.Context) error {
	log.Info("Org: Config")
	if c.NArg() != 2 {
		log.Fatal("Please give pop_desc.toml and group.toml")
	}
	cfg, client := getConfigClient(c)
	if cfg.Address.String() == "" {
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
	desc.Roster = group.Roster
	log.Info("Hash of config is:", base64.StdEncoding.EncodeToString(desc.Hash()))
	//log.ErrFatal(check.Servers(group), "Couldn't check servers")
	log.ErrFatal(client.StoreConfig(cfg.Address, desc))
	cfg.Final.Desc = desc
	cfg.write()
	return nil
}

// creates a new private/public pair
func clientCreate(c *cli.Context) error {
	priv := network.Suite.NewKey(random.Stream)
	pub := network.Suite.Point().Mul(nil, priv)
	privStr, err := crypto.ScalarToString64(nil, priv)
	if err != nil {
		return err
	}
	pubStr, err := crypto.PubToString64(nil, pub)
	if err != nil {
		return err
	}
	log.Infof("Private: %s\nPublic: %s", privStr, pubStr)
	return nil
}

// getConfigClient returns the configuration and a client-structure.
func getConfigClient(c *cli.Context) (*Config, *service.Client) {
	cfg, err := newConfig(path.Join(c.GlobalString("config"), "config.bin"))
	log.ErrFatal(err)
	return cfg, service.NewClient()
}

// newConfig tries to read the config and returns an organizer-
// config if it doesn't find anything.
func newConfig(fileConfig string) (*Config, error) {
	name := app.TildeToHome(fileConfig)
	if _, err := os.Stat(name); err != nil {
		kp := config.NewKeyPair(network.Suite)
		return &Config{
			Private: kp.Secret,
			Public:  kp.Public,
			Final: &service.FinalStatement{
				Attendees: []abstract.Point{},
				Signature: []byte{},
			},
			Index: -1,
			name:  name,
		}, nil
	}
	buf, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("couldn't read %s: %s - please remove it",
			name, err)
	}
	_, msg, err := network.Unmarshal(buf)
	if err != nil {
		return nil, fmt.Errorf("error while reading file %s: %s",
			name, err)
	}
	cfg, ok := msg.(*Config)
	if !ok {
		log.Fatal("Wrong data-structure in file", name)
	}
	cfg.name = name
	return cfg, nil
}

// write saves the config to the given file.
func (cfg *Config) write() {
	buf, err := network.Marshal(cfg)
	log.ErrFatal(err)
	log.ErrFatal(ioutil.WriteFile(cfg.name, buf, 0660))
}

// readGroup fetches group definition file.
func readGroup(name string) *app.Group {
	f, err := os.Open(name)
	log.ErrFatal(err, "Couldn't open group definition file")
	group, err := app.ReadGroupDescToml(f)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s",
			name)
	}
	return group
}
