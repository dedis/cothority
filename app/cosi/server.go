package main

import (
	"fmt"
	"os"

	c "github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/log"
	"gopkg.in/urfave/cli.v1"

	// Empty imports to have the init-functions called which should
	// register the protocol
	_ "github.com/dedis/cothority/protocols/cosi"
	_ "github.com/dedis/cothority/services/cosi"
)

// FIXME this is still the same code as in cothorityd!
// Do we really want to export runServer?
func runServer(ctx *cli.Context) {
	// first check the options
	config := ctx.String("config")

	if _, err := os.Stat(config); os.IsNotExist(err) {
		log.Fatalf("[-] Configuration file does not exist. %s. "+
			"Use `cosi server setup` to create one.", config)
	}
	// Let's read the config
	_, conode, err := c.ParseCothorityd(config)
	if err != nil {
		log.Fatal("Couldn't parse config:", err)
	}
	conode.Start()
}

// FIXME: user log:Fatal as in cothorityd instead?
func stderrExit(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, fmt.Sprintf(format, a...)+"\n")
	os.Exit(1)
}
