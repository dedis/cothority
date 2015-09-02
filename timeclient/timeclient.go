package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"

	"github.com/dedis/prifi/coco/test/logutils"
	"github.com/dedis/prifi/coco/test/oldconfig"
	"github.com/dedis/prifi/coco/test/timeclient/stampclient"
)

var server string
var nmsgs int
var name string
var logger string
var rate int
var debug bool

func init() {
	addr, _ := oldconfig.GetAddress()
	// TODO: change to take in list of servers: comma separated no spaces
	//   -server=s1,s2,s3,...
	flag.StringVar(&server, "server", "", "the timestamping servers to contact")
	flag.IntVar(&nmsgs, "nmsgs", 100, "messages per round")
	flag.StringVar(&name, "name", addr, "name for the client")
	flag.StringVar(&logger, "logger", "", "remote logger")
	flag.IntVar(&rate, "rate", -1, "milliseconds between timestamp requests")
	flag.BoolVar(&debug, "debug", false, "set debug mode")
	//log.SetFormatter(&log.JSONFormatter{})
}

func main() {
	flag.Parse()
	if logger != "" {
		// blocks until we can connect to the logger
		lh, err := logutils.NewLoggerHook(logger, name, "timeclient")
		if err != nil {
			log.Fatal(err)
		}
		log.AddHook(lh)
	}
	stampclient.Run(server, nmsgs, name, rate, debug)
}
