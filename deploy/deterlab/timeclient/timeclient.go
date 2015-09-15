package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/helpers/debug_lvl"
	"github.com/dedis/cothority/helpers/logutils"
	"github.com/dedis/cothority/helpers/oldconfig"
	"github.com/dedis/cothority/deploy/deterlab/timeclient/stampclient"
	"github.com/dedis/cothority/deploy"
)

var deter *deploy.Deter
var conf *deploy.Config
var server string
var name string
var logger string

func init() {
	addr, _ := oldconfig.GetAddress()
	// TODO: change to take in list of servers: comma separated no spaces
	//   -server=s1,s2,s3,...
	flag.StringVar(&server, "server", "", "the timestamping servers to contact")
	flag.StringVar(&name, "name", addr, "name for the client")
	flag.StringVar(&logger, "logger", "", "remote logger")
}

func main() {
	deter, err := deploy.ReadConfig()
	if err != nil {
		log.Fatal("Couldn't load config-file in timeclient:", err)
	}
	conf = deter.Config
	dbg.Lvl1("deter is", deter)
	dbg.Lvl1("conf is", conf)
	dbg.DebugVisible = conf.Debug

	flag.Parse()
	if logger != "" {
		// blocks until we can connect to the logger
		lh, err := logutils.NewLoggerHook(logger, name, "timeclient")
		if err != nil {
			log.Fatal(err)
		}
		log.AddHook(lh)
	}
	dbg.Lvl2("Timeclient starts")
	stampclient.Run(server, conf.Nmsgs, name, conf.Rate)
	dbg.Lvl2("Timeclient.go ", name, "main() ", name, " finished...")
}
