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

func stderr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, fmt.Sprintf(format, a...)+"\n")
}

func stderrExit(format string, a ...interface{}) {
	stderr(format, a...)
	os.Exit(1)
}
