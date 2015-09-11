package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"
	"github.com/ineiti/cothorities/helpers/logutils"
	"github.com/ineiti/cothorities/helpers/oldconfig"
	"github.com/ineiti/cothorities/platforms/deterlab/timeclient/stampclient"
)

var server string
var nmsgs int
var name string
var logger string
var rate int
var debug int

func init() {
	addr, _ := oldconfig.GetAddress()
	// TODO: change to take in list of servers: comma separated no spaces
	//   -server=s1,s2,s3,...
	flag.StringVar(&server, "server", "", "the timestamping servers to contact")
	flag.IntVar(&nmsgs, "nmsgs", 100, "messages per round")
	flag.StringVar(&name, "name", addr, "name for the client")
	flag.StringVar(&logger, "logger", "", "remote logger")
	flag.IntVar(&rate, "rate", -1, "milliseconds between timestamp requests")
	flag.IntVar(&debug, "debug", 0, "set debug mode-level")
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
	log.Println("Timeclient starts")
	stampclient.Run(server, nmsgs, name, rate, debug)
	log.Printf("Timeclient.go ", name, "main() ", name, " finished...")
}
