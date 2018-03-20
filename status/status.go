// Status takes in a file containing a list of servers and returns the status
// reports of all of the servers.  A status is a list of connections and
// packets sent and received for each server in the file.
package main

import (
	"errors"
	"os"
	"sort"
	"strings"

	status "github.com/dedis/cothority/status/service"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"gopkg.in/urfave/cli.v1"
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
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: `integer`: 1 for terse, 5 for maximal",
		},
	}
	app.Action = func(c *cli.Context) error {
		log.SetUseColors(false)
		log.SetDebugVisible(c.GlobalInt("debug"))
		return network(c)
	}
	app.Run(os.Args)
}

// network will contact all cothorities in the group-file and print
// the status-report of each one.
func network(c *cli.Context) error {
	groupToml := c.GlobalString("g")
	el, err := readGroup(groupToml)
	log.ErrFatal(err, "Couldn't Read File")
	log.Lvl3(el)
	cl := status.NewClient()
	for i := 0; i < len(el.List); i++ {
		log.Lvl3(el.List[i])
		sr, err := cl.Request(el.List[i])
		if err != nil {
			log.Print("error on server", el.List[i], err)
			continue
		}
		printConn(sr)
		log.Lvl3(cl)
	}
	return nil
}

// readGroup takes a toml file name and reads the file, returning the entities within
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

// printConn prints the status response that is returned from the server
func printConn(e *status.Response) {
	var a []string
	if e.Status == nil {
		log.Print("no status from ", e.ServerIdentity)
		return
	}

	for sec, st := range e.Status {
		for key, value := range st.Field {
			a = append(a, (sec + "." + key + ": " + value))
		}
	}
	sort.Strings(a)
	log.Print(strings.Join(a, "\n"))
}
