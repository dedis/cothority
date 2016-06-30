// Status takes in a file containing a list of servers and returns the status reports of all of the servers.
// A status is a list of connections and packets sent and received for each server in the file.
package main

import (
	"os"

	"errors"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"

	"sort"
	"strings"

	"github.com/dedis/cothority/services/status"
	"gopkg.in/codegangsta/cli.v1"
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
		sr, _ := cl.GetStatus(el.List[i])
		printConn(sr)
		log.Lvl3(cl)
	}
	return nil
}

// readGroup takes a toml file name and reads the file, returning the entities within
func readGroup(tomlFileName string) (*sda.Roster, error) {
	log.Print("Reading From File")
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	el, err := config.ReadGroupToml(f)
	if err != nil {
		return nil, err
	}
	if len(el.List) <= 0 {
		return nil, errors.New("Empty or invalid group file:" +
			tomlFileName)
	}
	log.Lvl3(el)
	return el, err
}

// printConn prints the status response that is returned from the server
func printConn(e *status.Response) {
	var a []string
	for key, value := range e.Msg["Status"] {
		a = append(a, (key + ": " + value + "\n"))
	}
	sort.Strings(a)
	strings.Join(a, "\n")
	log.Print(a)
}
