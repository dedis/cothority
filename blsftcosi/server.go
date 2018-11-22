package main

import (
	_ "github.com/dedis/cothority/blsftcosi/protocol"
	_ "github.com/dedis/cothority/blsftcosi/service"
	"github.com/dedis/onet/app"
	"gopkg.in/urfave/cli.v1" // Empty imports to have the init-functions called which should
	// register the protocol
)

func runServer(ctx *cli.Context) {
	// first check the options
	config := ctx.String("config")

	app.RunServer(config)
}
