// Cosi takes a file or a message and signs it collectively.
// For usage, see README.md
package main

import (
	"os"

	"errors"
	"time"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/app/lib/ui"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"

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
	dbg.SetDebugVisible(1)
	dbg.SetUseColors(false)
}

func main() {
	app := cli.NewApp()
	app.Name = "Status"
	app.Usage = "Get and print status of all servers in a file."
	app.Commands = []cli.Command{
		{
			Name:    "getStatus",
			Aliases: []string{"g"},
			Usage:   "Gets status from all entities in group file.",
			Action:  getStatus,
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
		dbg.SetDebugVisible(c.GlobalInt("debug"))
		return nil
	}
	app.Run(os.Args)
}

// signFile will search for the file and sign it
// it always returns nil as an error
func getStatus(c *cli.Context) error {
	groupToml := c.GlobalString(optionGroup)
	el, err := readGroup(groupToml)
	ui.ErrFatal(err, "Couldn't read file.")
	dbg.Lvl3(el)
	cl := status.NewClient()
	for i := 0; i < len(el.List); i++ {
		dbg.Print("Hello: ")
		dbg.Print(el.List[i])
		sr, _ := cl.GetStatus(el.List[i])
		printConn(sr)
		dbg.Print(cl)
	}
	return nil
}

// sign takes a stream and a toml file defining the servers
func readGroup(tomlFileName string) (*sda.EntityList, error) {
	dbg.Lvl2("Reading From File")
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
	dbg.Print(el)
	return el, err
}

func printConn(e *status.StatusResponse) {
	fmt.Print("Host: ", e.Serv, "\n")
	for i := 0; i < len(e.Received); i++ {
		fmt.Print("\n")
		fmt.Print("Connection: ", e.Remote[i], "\n")
		fmt.Print("Total Packets Recieved: ", e.Received[i], "\n")
		fmt.Print("Total Packets Sent: ", e.Sent[i], "\n")
		fmt.Print("\n")
	}
	fmt.Print("\n \n")
}

/*
func write(r io.Writer, tomlFileName string) {
	dbg.Lvl2("Reading From File")
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}

}
*/
