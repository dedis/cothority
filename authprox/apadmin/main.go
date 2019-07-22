package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/authprox"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	cli "gopkg.in/urfave/cli.v1"
)

var cmds = cli.Commands{
	{
		Name:  "add",
		Usage: "add an external identity provider",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "roster, r",
				Usage: "the roster of the cothority that hosts the distributed Authentication Proxy",
			},
			cli.StringFlag{
				Name:  "type",
				Usage: "the type of validator: oidc (OpenID Connect)",
				Value: "oidc",
			},
			cli.StringFlag{
				Name:  "issuer",
				Usage: "the URL of the identity provider (for OpenID Connect, this is the Issuer URL)",
			},
		},
		Action: add,
	},
	{
		Name:  "show",
		Usage: "show the enrollments of external identity providers",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "roster, r",
				Usage: "the roster of the cothority that hosts the distributed Authentication Proxy",
			},
		},
		Action: show,
	},
}

var cliApp = cli.NewApp()

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

func init() {
	cliApp.Name = "apadmin"
	cliApp.Usage = "Administer authentication proxies."
	cliApp.Version = "0.1"
	cliApp.Commands = cmds
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "",
			Usage: "path to configuration-directory",
		},
		cli.BoolFlag{
			Name:   "wait, w",
			EnvVar: "BC_WAIT",
			Usage:  "wait for transaction available in all nodes",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		lib.ConfigPath = c.String("config")
		if lib.ConfigPath == "" {
			lib.ConfigPath = getDataPath(cliApp.Name)
		}
		return nil
	}
}

func main() {
	log.ErrFatal(cliApp.Run(os.Args))
}

func show(c *cli.Context) error {
	fn := c.String("roster")
	if fn == "" {
		return errors.New("--roster flag is required")
	}

	in, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("Could not open roster %v: %v", fn, err)
	}
	roster, err := readRoster(in)
	if err != nil {
		return err
	}
	cl := onet.NewClient(cothority.Suite, authprox.ServiceName)
	req := &authprox.EnrollmentsRequest{}
	resp := &authprox.EnrollmentsResponse{}
	err = cl.SendProtobuf(roster.List[0], req, resp)
	if err != nil {
		return err
	}

	for _, x := range resp.Enrollments {
		fmt.Println(x.Type, x.Issuer, x.Public)
	}
	return nil
}

func add(c *cli.Context) error {
	is := c.String("issuer")
	if is == "" {
		return errors.New("--issuer flag is required")
	}

	// Load the roster
	fn := c.String("roster")
	if fn == "" {
		return errors.New("--roster flag is required")
	}

	in, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("Could not open roster %v: %v", fn, err)
	}
	roster, err := readRoster(in)
	if err != nil {
		return err
	}

	// Get all the key material ready.

	// n: how many auth proxies will be holding shares
	n := len(roster.List)
	T := threshold(n)

	pubs := make([]kyber.Point, n)
	for i := range pubs {
		pubs[i] = roster.List[i].Public
	}

	lPri := share.NewPriPoly(cothority.Suite, T, nil, cothority.Suite.RandomStream())
	lShares := lPri.Shares(n)
	lPub := lPri.Commit(nil)
	_, lPubCommits := lPub.Info()

	for i, s := range roster.List {
		if !(isLoopback(s.Address) || isHTTPS(s.URL)) {
			return fmt.Errorf("cannot send secret over insecure channel to %v", s)
		}
		cl := onet.NewClient(cothority.Suite, authprox.ServiceName)
		lpri := authprox.PriShare{
			I: lShares[i].I,
			V: lShares[i].V,
		}
		req := &authprox.EnrollRequest{
			Type:         c.String("type"),
			Issuer:       is,
			Participants: pubs,
			LongPri:      lpri,
			LongPubs:     lPubCommits,
		}
		resp := &authprox.EnrollResponse{}
		err := cl.SendProtobuf(s, req, resp)
		if err != nil {
			return fmt.Errorf("cannot enroll with %v: %v", s, err)
		}
	}

	fmt.Fprintln(c.App.Writer, "External provider enrolled. Use identities of this form:")
	fmt.Fprintf(c.App.Writer, "\tproxy:%v:user@example.com\n", lPubCommits[0])
	return nil
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

func faultThreshold(n int) int {
	return (n - 1) / 3
}

func threshold(n int) int {
	return n - faultThreshold(n)
}

func isLoopback(a network.Address) bool {
	ad := a.Resolve()
	ip := net.ParseIP(ad)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func isHTTPS(ustr string) bool {
	u, err := url.Parse(ustr)
	if err != nil {
		return false
	}
	return u.Scheme == "https"
}
