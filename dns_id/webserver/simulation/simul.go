package main

import (
	_ "github.com/dedis/cothority/dns_id/webserver"
	"gopkg.in/dedis/onet.v1/simul"
)

func main() {
	simul.Start()
}
