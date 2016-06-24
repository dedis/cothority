// Cosi takes a file or a message and signs it collectively.
// For usage, see README.md
package main

import (
	"os"

	"errors"
	"time"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"

	"fmt"

	"github.com/dedis/cothority/services/status"
	"gopkg.in/codegangsta/cli.v1"
)

// RequestTimeOut defines when the client stops waiting for the CoSi group to
// reply
const RequestTimeOut = time.Second * 10

const optionGroup = "group"
const optionGroupShort = "g"

func init() {
	log.SetDebugVisible(1)
	log.SetUseColors(false)
}

func main() {
	app := cli.NewApp()
	app.Name = "Status"
	app.Usage = "Get and print status of all servers in a file."
	//a status is a list of connections and packets sent and received for each server in the file
	app.Commands = []cli.Command{
		{
			Name:    "network",
			Aliases: []string{"g"},
			Usage:   "Gets status from all entities in group file.",
			Action:  network,
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  optionGroup + " ," + optionGroupShort,
			Value: "group.toml",
			Usage: "Cothority group definition in `FILE.toml`",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: `integer`: 1 for terse, 5 for maximal",
		},
	}
	app.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.GlobalInt("debug"))
		return nil
	}
	app.Action = network
	app.Run(os.Args)
}

// signFile will search for the file and sign it
// it always returns nil as an error
func network(c *cli.Context) error {
	groupToml := c.GlobalString(optionGroup)
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

// sign takes a stream and a toml file defining the servers
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

func printConn(e *status.Response) {
	log.Print("Host: ", e.Serv)
	for i := 0; i < len(e.Received); i++ {
		log.Print("\nConnection: ", e.Remote[i])
		log.Print("Total Packets Recieved: ", e.Received[i])
		log.Print("Total Packets Sent: ", e.Sent[i])
		log.Print("Services Available: ", e.Available)
	}
	fmt.Print("\n")
}
