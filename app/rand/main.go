package main

import (
	"flag"
)

func main() {
	server := false

	flag.BoolVar(&server, "server", false, "Start server")
	flag.Parse()

	if server {
		panic("server")
	} else {
	}
}
