package main

import (
	"github.com/urfave/cli"

	// Empty imports to have the init-functions called which should
	// register the protocol

	_ "go.dedis.ch/cothority/v3/ftcosi/protocol"
	_ "go.dedis.ch/cothority/v3/ftcosi/service"
	"go.dedis.ch/onet/v3/app"
)

func runServer(ctx *cli.Context) {
	// first check the options
	config := ctx.String("config")

	app.RunServer(config)
}
