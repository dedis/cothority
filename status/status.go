// Status takes in a file containing a list of servers and returns the status
// reports of all of the servers.  A status is a list of connections and
// packets sent and received for each server in the file.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	cli "github.com/urfave/cli"
	status "go.dedis.ch/cothority/v3/status/service"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

func main() {
	app := cli.NewApp()
	app.Name = "Status"
	app.Usage = "Get and print status of all servers of a file."

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "group, g",
			Value: "group.toml",
			Usage: "Cothority group definition in `FILE.toml`",
		},
		cli.StringFlag{
			Name:  "host",
			Usage: "Request information about this host",
		},
		cli.StringFlag{
			Name:  "format, f",
			Value: "txt",
			Usage: "Output format: \"txt\" (default) or \"json\".",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: `integer`: 1 for terse, 5 for maximal",
		},
		cli.BoolFlag{
			Name: "connectivity, c",
		},
	}
	app.Commands = cli.Commands{
		{
			Name:      "connectivity",
			Usage:     "if given, will verify connectivity of all nodes between themselves",
			Aliases:   []string{"c"},
			ArgsUsage: "group.toml private.toml",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "findFaulty, f",
					Usage: "it tries to find a list of nodes that can communicate with each other",
				},
				cli.StringFlag{
					Name:  "timeout, to",
					Usage: "timeout in ms to wait if a set of nodes is connected",
					Value: "1s",
				},
			},
			Action: connectivity,
		},
	}
	app.Action = func(c *cli.Context) error {
		log.SetUseColors(false)
		log.SetDebugVisible(c.GlobalInt("debug"))
		return action(c)
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

type se struct {
	Server *network.ServerIdentity
	Status *status.Response
	Err    string
}

// will contact all cothorities in the group-file and print
// the status-report of each one.
func action(c *cli.Context) error {
	groupToml := c.GlobalString("g")
	format := c.String("format")
	var list []*network.ServerIdentity

	host := c.String("host")
	if host != "" {
		// Only contact one host
		log.Info("Only contacting one host", host)
		addr := network.Address(host)
		if !strings.HasPrefix(host, "tls://") {
			addr = network.NewAddress(network.TLS, host)
		}
		si := network.NewServerIdentity(nil, addr)
		if si.Address.Port() == "" {
			return errors.New("port not found, must provide host:port")
		}
		list = append(list, si)
	} else {

		ro, err := readGroup(groupToml)
		if err != nil {
			return errors.New("couldn't read file: " + err.Error())
		}
		log.Lvl3(ro)
		list = ro.List
		log.Info("List is", list)
	}
	cl := status.NewClient()

	var all []se
	for _, server := range list {
		sr, err := cl.Request(server)
		if err != nil {
			err = fmt.Errorf("could not get status from %v: %v", server, err)
		}

		if format == "txt" {
			if err != nil {
				log.Error(err)
			} else {
				printTxt(sr)
			}
		} else {
			// JSON
			errStr := "ok"
			if err != nil {
				errStr = err.Error()
			}
			all = append(all, se{Server: server, Status: sr, Err: errStr})
		}
	}
	if format == "json" {
		printJSON(all)
	}
	return nil
}

func connectivity(c *cli.Context) error {
	if c.NArg() != 2 {
		return errors.New("please give 2 arguments: group.toml private.toml")
	}
	ro, err := readGroup(c.Args().First())
	if err != nil {
		return errors.New("couldn't read file: " + err.Error())
	}
	log.Lvl3(ro)
	list := ro.List
	log.Info("List is", list)
	to, err := time.ParseDuration(c.String("timeout"))
	if err != nil {
		return errors.New("duration parse error: " + err.Error())
	}
	ff := c.Bool("findFaulty")
	coth, err := app.LoadCothority(c.Args().Get(1))
	if err != nil {
		return errors.New("error while loading private.toml: " + err.Error())
	}
	si, err := coth.GetServerIdentity()
	if err != nil {
		return errors.New("private.toml didn't have a serverIdentity: " + err.Error())
	}
	resp, err := status.NewClient().CheckConnectivity(si.GetPrivate(), list, time.Duration(to), ff)
	if err != nil {
		return errors.New("couldn't get private key from private.toml: " + err.Error())
	}
	switch len(resp) {
	case 1:
		return errors.New("couldn't contact any other node")
	case len(list):
		log.Info("All nodes can communicate with each other")
	default:
		log.Info("The following nodes can communicate with each other")
	}
	for _, si := range resp {
		log.Info("  ", si.String())
	}
	return nil
}

// readGroup takes a toml file name and reads the file, returning the entities
// within.
func readGroup(tomlFileName string) (*onet.Roster, error) {
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	g, err := app.ReadGroupDescToml(f)
	if err != nil {
		return nil, err
	}
	if len(g.Roster.List) <= 0 {
		return nil, errors.New("Empty or invalid group file:" +
			tomlFileName)
	}
	log.Lvl3(g.Roster)
	return g.Roster, err
}

// prints the status response that is returned from the server
func printTxt(e *status.Response) {
	log.Info("-----------------------------------------------")
	log.Infof("Address = \"%s\"", e.ServerIdentity.Address)
	log.Info("Suite = \"Ed25519\"")
	log.Infof("Public = \"%s\"", e.ServerIdentity.Public)
	log.Infof("Description = \"%s\"", e.ServerIdentity.Description)
	log.Info("-----------------------------------------------")
	var a []string
	if e.Status == nil {
		log.Error("no status from ", e.ServerIdentity)
		return
	}

	for sec, st := range e.Status {
		for key, value := range st.Field {
			a = append(a, (sec + "." + key + ": " + value))
		}
	}
	sort.Strings(a)
	log.Info(strings.Join(a, "\n"))
}

func printJSON(all []se) {
	b1 := new(bytes.Buffer)
	e := json.NewEncoder(b1)
	e.Encode(all)

	b2 := new(bytes.Buffer)
	json.Indent(b2, b1.Bytes(), "", "  ")

	out := bufio.NewWriter(os.Stdout)
	out.Write(b2.Bytes())
	out.Flush()
}
