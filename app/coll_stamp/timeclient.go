package coll_stamp

import (
	"flag"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/logutils"
	"github.com/dedis/cothority/deploy"
)

var deter *deploy.Deter
var conf *deploy.Config
var name string

func RunClientSetup(server, logger string) {
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
	RunClient(server, conf.Nmsgs, name, conf.Rate)
	dbg.Lvl2("Timeclient.go ", name, "main() ", name, " finished...")
}
