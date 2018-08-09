package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/eventlog"
	"github.com/dedis/cothority/omniledger/darc"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/kyber/util/encoding"
	"github.com/dedis/onet/log"
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
			cli.StringFlag{
				Name:   "priv",
				EnvVar: "PRIVATE_KEY",
				Usage:  "the ed25519 private key that will sign the create transaction",
			},
			cli.StringFlag{
				Name:   "ol",
				EnvVar: "OL",
				Usage:  "the OmniLedger config",
			},
		},
		Action: create,
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
				Name:   "el",
				EnvVar: "EL",
				Usage:  "the eventlog id (64 hex bytes), from \"el create\"",
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
				Name:   "el",
				EnvVar: "EL",
				Usage:  "the eventlog id (64 hex bytes), from \"el create\"",
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
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
}

func main() {
	log.ErrFatal(cliApp.Run(os.Args))
}

// getClient will create a new eventlog.Client, given the input
// available in the commandline. If priv is false, then it will not
// look for a private key and set up the signers. (This is used for
// searching, which does not require having a private key available
// because it does not submit transactions.)
func getClient(c *cli.Context, priv bool) (*eventlog.Client, error) {
	fn := c.String("ol")
	if fn == "" {
		return nil, errors.New("--ol is required")
	}
	ol, err := omniledger.NewClientFromConfig(fn)
	if err != nil {
		return nil, err
	}
	cl := eventlog.NewClient(ol)

	d, err := cl.OmniLedger.GetGenDarc()
	if err != nil {
		return nil, err
	}
	cl.DarcID = d.GetBaseID()

	if priv {
		privStr := c.String("priv")
		if privStr == "" {
			return nil, errors.New("--priv is required")
		}
		priv, err := encoding.StringHexToScalar(cothority.Suite, privStr)
		if err != nil {
			return nil, err
		}
		pub := cothority.Suite.Point().Mul(priv, nil)

		cl.Signers = []darc.Signer{darc.NewSignerEd25519(pub, priv)}
	}
	return cl, nil
}

func create(c *cli.Context) error {
	if c.Bool("keys") {
		s := darc.NewSignerEd25519(nil, nil)
		fmt.Println("Identity:", s.Identity())
		fmt.Printf("export PRIVATE_KEY=%v\n", s.Ed25519.Secret)
		return nil
	}

	cl, err := getClient(c, true)
	if err != nil {
		return err
	}

	genDarc, err := cl.OmniLedger.GetGenDarc()
	if err != nil {
		return err
	}
	cl.DarcID = genDarc.GetBaseID()

	err = cl.Create()
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "export EL=%x\n", cl.Instance.Slice())
	return nil
}

func doLog(c *cli.Context) error {
	cl, err := getClient(c, true)
	if err != nil {
		return err
	}
	e := c.String("el")
	if e == "" {
		return errors.New("--el is required")
	}
	eb, err := hex.DecodeString(e)
	if err != nil {
		return err
	}
	cl.Instance = omniledger.NewInstanceID(eb)

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
	req := &eventlog.SearchRequest{
		Topic: c.String("topic"),
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

	cl, err := getClient(c, false)
	if err != nil {
		return err
	}
	e := c.String("el")
	if e == "" {
		return errors.New("--el is required")
	}
	eb, err := hex.DecodeString(e)
	if err != nil {
		return err
	}
	cl.Instance = omniledger.NewInstanceID(eb)

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
