package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/eventlog"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/cfgpath"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

type config struct {
	Name   string
	ID     skipchain.SkipBlockID
	Roster *onet.Roster
	Owner  *darc.Signer
	Darc   *darc.Darc
}

var cmds = cli.Commands{
	{
		Name:    "create",
		Usage:   "create an event log",
		Aliases: []string{"c"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "roster, r",
				Usage: "the roster of the cothority that will host the event log",
			},
			cli.StringFlag{
				Name:  "name, n",
				Usage: "the name of this config (for display only)",
			},
		},
		Action: create,
	},
	{
		Name:    "show",
		Usage:   "show the known configs",
		Aliases: []string{"s"},
		Action:  show,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "long, l",
				Usage: "long listing",
			},
		},
	},
	{
		Name:    "log",
		Usage:   "log one or more messages",
		Aliases: []string{"l"},
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:   "config",
				Value:  1,
				EnvVar: "EL_CONFIG",
				Usage:  "config number to use",
			},
			cli.StringFlag{
				Name:  "topic, t",
				Usage: "the topic of the log",
			},
			cli.StringFlag{
				Name:  "content, c",
				Usage: "the text of the log",
			},
		},
		Action: doLog,
	},
}

func init() {
	network.RegisterMessages(&config{})
}

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "el"
	cliApp.Usage = "Create and work with OmniLedger event logs."
	cliApp.Version = "0.1"
	cliApp.Commands = cmds
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name: "config, c",
			// we use GetDataPath because only non-human-readable
			// data files are stored here
			Value: cfgpath.GetDataPath("scmgr"),
			Usage: "path to config-file",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	log.ErrFatal(cliApp.Run(os.Args))
}

func create(c *cli.Context) error {
	fn := c.String("roster")
	if fn == "" {
		return errors.New("-roster flag is required")
	}

	in, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("Could not open roster %v: %v", fn, err)
	}
	r, err := readRoster(in)
	if err != nil {
		return err
	}

	n := c.String("name")
	cfg, err := doCreate(n, r)
	if err != nil {
		return err
	}
	fmt.Printf("Created event log chain with ID %x.\n", cfg.ID)
	return err
}

func doCreate(name string, r *onet.Roster) (*config, error) {
	owner := darc.NewSignerEd25519(nil, nil)
	c := eventlog.NewClient(r)
	err := c.Init(owner, 5*time.Second)
	if err != nil {
		return nil, err
	}

	cfg := &config{
		Name:   name,
		ID:     c.ID,
		Roster: r,
		Owner:  owner,
		Darc:   c.Darc,
	}
	err = cfg.save()
	return cfg, err
}

func show(c *cli.Context) error {
	long := c.Bool("long")
	dir := getDataPath("el")
	cfgs, err := loadConfigs(dir)
	if err != nil {
		return err
	}
	for i, x := range cfgs {
		name := x.Name
		// Write the owner private key and the ID into the config file.
		if name == "" {
			name = "(none)"
		}
		if long {
			fmt.Printf("%2v: Name: %v, ID: %x\n", i+1, name, x.ID)
		} else {
			fmt.Printf("%2v: Name: %v, ID: %x\n", i+1, name, x.ID[0:8])
		}
	}
	return nil
}

func doLog(c *cli.Context) error {
	cfg := c.Int("config")

	// In the UI they are 1-based, but in cfg[] they are 0-based.
	cfg = cfg - 1

	cfgs, err := loadConfigs(getDataPath("el"))
	if err != nil {
		return err
	}
	if cfg > len(cfgs)-1 {
		return fmt.Errorf("config number %v is too big", cfg+1)
	}

	cl := eventlog.NewClient(cfgs[cfg].Roster)
	cl.ID = cfgs[cfg].ID
	// TODO: It should be possible to send logs, signing them with a different
	// key. But first, we need to implement something like "el grant" to grant write
	// privs to a given private/public key.
	cl.Signers = []*darc.Signer{cfgs[cfg].Owner}
	// TODO: It is too bad that we need to store the Darc in config. It
	// seems like the server should know this for us...
	cl.Darc = cfgs[cfg].Darc

	t := c.String("topic")

	content := c.String("content")

	// Content is set, so one shot log.
	if content != "" {
		_, err := cl.Log(eventlog.NewEvent(t, content))
		return err
	}

	// Content is empty, so read from stdin.
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		_, err := cl.Log(eventlog.NewEvent(t, s.Text()))
		if err != nil {
			return err
		}
	}
	return nil
}

func loadConfigs(dir string) ([]config, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	c := make([]config, len(files))
	ct := 0
	for _, f := range files {
		fn := filepath.Join(dir, f.Name())
		if filepath.Ext(fn) != ".cfg" {
			continue
		}
		v, err := ioutil.ReadFile(fn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %v: %v\n", fn, err)
			continue
		}
		_, val, err := network.Unmarshal(v, cothority.Suite)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unmarshal %v: %v\n", fn, err)
			continue
		}
		c[ct] = *(val.(*config))
		ct++
	}
	if ct == 0 {
		return nil, nil
	}
	return c[0:ct], nil
}

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

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

func (cfg *config) save() error {
	cfgDir := getDataPath("el")
	os.MkdirAll(cfgDir, 0755)

	fn := fmt.Sprintf("%x.cfg", cfg.ID[0:8])
	fn = filepath.Join(cfgDir, fn)

	// perms = 0400 because there is key material inside this file.
	f, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0400)
	if err != nil {
		return fmt.Errorf("could not write %v: %v", fn, err)
	}

	buf, err := network.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = f.Write(buf)
	if err != nil {
		return err
	}
	return f.Close()
}
