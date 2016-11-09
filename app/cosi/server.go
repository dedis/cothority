package main

import (
	"github.com/dedis/cothority/app/lib/server"
	"gopkg.in/urfave/cli.v1"

	// Empty imports to have the init-functions called which should
	// register the protocol
	_ "github.com/dedis/cothority/protocols/cosi"
	_ "github.com/dedis/cothority/services/cosi"
)

func runServer(ctx *cli.Context) {
	// first check the options
	config := ctx.String("config")

	server.RunServer(config)
}
