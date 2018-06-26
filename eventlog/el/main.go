package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/eventlog"
	"github.com/dedis/cothority/omniledger/darc"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/kyber/util/encoding"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet/cfgpath"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

type config struct {
	Name       string
	EventLogID omniledger.InstanceID
}

var cmds = cli.Commands{
	{
		Name:    "create",
		Usage:   "create an event log",
		Aliases: []string{"c"},
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "keys",
				Usage: "make a key pair",
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
			cli.StringFlag{
				Name:   "priv",
				EnvVar: "PRIVATE_KEY",
				Usage:  "the ed25519 private key that will sign transactions",
			},
			cli.StringFlag{
				Name:   "ol",
				EnvVar: "OL",
				Usage:  "the OmniLedger config",
			},
			cli.StringFlag{
				Name:  "eid",
				Usage: "the eventlog id (64 hex bytes)",
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
	{
		Name:    "search",
		Usage:   "search for messages",
		Aliases: []string{"s"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "ol",
				EnvVar: "OL",
				Usage:  "the OmniLedger config",
			},
			cli.StringFlag{
				Name:  "eid",
				Usage: "the eventlog id (64 hex bytes)",
			},
			cli.StringFlag{
				Name:  "topic, t",
				Usage: "limit results to logs with this topic",
			},
			cli.IntFlag{
				Name:  "count, c",
				Usage: "limit results to X events",
			},
			cli.StringFlag{
				Name:  "from",
				Usage: "return events from this time (accepts mm-dd-yyyy or relative times like '10m ago')",
			},
			cli.StringFlag{
				Name:  "to",
				Usage: "return events to this time (accepts mm-dd-yyyy or relative times like '10m ago')",
			},
			cli.DurationFlag{
				Name:  "for",
				Usage: "return events for this long after the from time (when for is given, to is ignored)",
			},
		},
		Action: search,
	},
}

var cliApp = cli.NewApp()

func init() {
	network.RegisterMessages(&config{})

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
}

func main() {
	log.ErrFatal(cliApp.Run(os.Args))
}

func create(c *cli.Context) error {
	if c.Bool("keys") {
		kp := key.NewKeyPair(cothority.Suite)
		fmt.Println("Private: ", kp.Private)
		fmt.Println(" Public: ", kp.Public)
		return nil
	}

	return errors.New("not implemented")
}

func show(c *cli.Context) error {
	return errors.New("not implemented")
	// long := c.Bool("long")
	// dir := getDataPath("el")
	// cfgs, err := loadConfigs(dir)
	// if err != nil {
	// 	return err
	// }
	// for i, x := range cfgs {
	// 	name := x.Name
	// 	if name == "" {
	// 		name = "(none)"
	// 	}
	// 	if long {
	// 		fmt.Printf("%2v: Name: %v, ID: %x\n", i+1, name, x.ID)
	// 	} else {
	// 		fmt.Printf("%2v: Name: %v, ID: %x\n", i+1, name, x.ID[0:8])
	// 	}
	// }
	// return nil
}

func doLog(c *cli.Context) error {
	fn := c.String("ol")
	if fn == "" {
		return errors.New("--ol is required")
	}
	ol, err := omniledger.NewClientFromConfig(fn)
	if err != nil {
		return err
	}
	cl := eventlog.NewClient(ol)
	cl.Signers = make([]darc.Signer, 1)

	privStr := c.String("priv")
	if privStr == "" {
		return errors.New("--priv is required")
	}
	priv, err := encoding.StringHexToScalar(cothority.Suite, privStr)
	if err != nil {
		return err
	}
	pub := cothority.Suite.Point().Mul(priv, nil)

	cl.Signers[0] = darc.NewSignerEd25519(pub, priv)

	e := c.String("eid")
	if e == "" {
		return errors.New("--eid is required")
	}
	eb, err := hex.DecodeString(e)
	if err != nil {
		return err
	}
	cl.InstanceID = omniledger.BytesToObjID(eb)

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

var none = time.Unix(0, 0)

// parseTime will accept either dates or "X ago" where X is a duration.
func parseTime(in string) (time.Time, error) {
	if strings.HasSuffix(in, " ago") {
		in = strings.Replace(in, " ago", "", -1)
		d, err := time.ParseDuration(in)
		if err != nil {
			return none, err
		}
		return time.Now().Add(-1 * d), nil
	}
	tm, err := time.Parse("01-02-2006", in)
	if err != nil {
		return none, err
	}
	return tm, nil
}

func search(c *cli.Context) error {
	fn := c.String("ol")
	if fn == "" {
		return errors.New("--ol is required")
	}
	ol, err := omniledger.NewClientFromConfig(fn)
	if err != nil {
		return err
	}

	e := c.String("eid")
	if e == "" {
		return errors.New("--eid is required")
	}
	eb, err := hex.DecodeString(e)
	if err != nil {
		return err
	}
	eid := omniledger.BytesToObjID(eb)

	req := &eventlog.SearchRequest{
		EventLogID: eid,
		Topic:      c.String("topic"),
	}

	f := c.String("from")
	if f != "" {
		ft, err := parseTime(f)
		if err != nil {
			return err
		}
		req.From = ft.UnixNano()
	}

	forDur := c.Duration("for")
	if forDur == 0 {
		// No -for, parse -to.
		t := c.String("to")
		if t != "" {
			tt, err := parseTime(t)
			if err != nil {
				return err
			}
			req.To = tt.UnixNano()
		}
	} else {
		// Parse -for
		req.To = time.Unix(0, req.From).Add(forDur).UnixNano()
	}

	cl := eventlog.NewClient(ol)
	resp, err := cl.Search(req)
	if err != nil {
		return err
	}

	ct := c.Int("count")

	for _, x := range resp.Events {
		const tsFormat = "2006-01-02 15:04:05"
		fmt.Fprintf(c.App.Writer, "%v\t%v\t%v\n", time.Unix(0, x.When).Format(tsFormat), x.Topic, x.Content)

		if ct != 0 {
			ct--
			if ct == 0 {
				break
			}
		}
	}

	if resp.Truncated {
		return cli.NewExitError("", 1)
	}
	return nil
}

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

func (c *config) save() error {
	cfgDir := getDataPath("el")
	os.MkdirAll(cfgDir, 0755)

	fn := fmt.Sprintf("%x.cfg", c.ID[0:8])
	fn = filepath.Join(cfgDir, fn)

	// perms = 0400 because there is key material inside this file.
	f, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0400)
	if err != nil {
		return fmt.Errorf("could not write %v: %v", fn, err)
	}

	buf, err := network.Marshal(c)
	if err != nil {
		return err
	}
	_, err = f.Write(buf)
	if err != nil {
		return err
	}
	return f.Close()
}
