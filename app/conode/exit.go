package main

/* Sends the 'exit'-command to a certain conode in the hope that he will stop,
 * update to the newest version, and restart.
 */

import (
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"golang.org/x/net/context"
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
	host := network.NewTcpHost(nil)
	conn, err := host.Open(add)
	if err != nil {
		dbg.Fatal("Could not connect to", add)
	}
	dbg.Lvl1("Connected to", add)
	msg := &conode.StampExit{}

	dbg.Lvl1("Asking to exit")
	ctx := context.TODO()
	err = conn.Send(ctx, msg)
	if err != nil {
		dbg.Fatal("Couldn't send exit-message to server:", err)
	}
}
