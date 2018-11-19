package main

import (
	"gopkg.in/urfave/cli.v1"

	// Empty imports to have the init-functions called which should
	// register the protocol

	"github.com/dedis/onet/app"
	_ "github.com/dedis/student_18_blsftcosi/blsftcosi/protocol"
	_ "github.com/dedis/student_18_blsftcosi/blsftcosi/service"
)

func runServer(ctx *cli.Context) {
	// first check the options
	config := ctx.String("config")

	app.RunServer(config)
}
