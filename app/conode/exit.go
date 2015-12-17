package main

/* Sends the 'exit'-command to a certain conode in the hope that he will stop,
 * update to the newest version, and restart.
 */

import (
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
)

func init() {
	command := cli.Command{
		Name:        "exit",
		Aliases:     []string{"x"},
		Usage:       "Stops the given CoNode",
		Description: "Basically it will statically generate the tree, with the respective names and public key.",
		ArgsUsage:   "ADDRESS: the IPv4[:PORT] of the CoNode to exit.",
		Action: func(c *cli.Context) {
			if c.Args().First() == "" {
				dbg.Fatal("You must provide an address")
			}
			ForceExit(c.Args().First())
		},
	}
	registerCommand(command)
}

// ForceExit connects to the stamp-port of the conode and asks him to exit
func ForceExit(address string) {
	add, err := cliutils.VerifyPort(address, conode.DefaultPort+1)
	if err != nil {
		dbg.Fatal("Couldn't convert", address, "to a IP:PORT")
	}

	conn := coconet.NewTCPConn(add)
	err = conn.Connect()
	if err != nil {
		dbg.Fatal("Error when getting the connection to the host:", err)
	}
	dbg.Lvl1("Connected to", add)
	msg := &conode.TimeStampMessage{
		Type: conode.StampExit,
	}

	dbg.Lvl1("Asking to exit")
	var data coconet.BinaryMarshaler
	data = msg
	err = conn.PutData(data)
	if err != nil {
		dbg.Fatal("Couldn't send exit-message to server:", err)
	}
}
