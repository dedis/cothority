package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"io/ioutil"
	"os"
	"time"
)

func main() {

	conf := new(app.NTreeConfig)
	app.ReadConfig(conf)

	// we must know who we are
	if app.RunFlags.Hostname == "" {
		log.Fatal("Hostname empty: Abort")
	}

	own, depth := conf.Tree.FindByName(app.RunFlags.Hostname, 0)
	if depth == 0 {
		// i.e. we are root
		conf.Root = true
	}
	if own == nil {
		dbg.Fatal("Could not find its name in the tree", app.RunFlags.Hostname)
	}
	conf.Tree = own
	conf.Name = own.Name
	// Wait for everybody to be ready before going on
	ioutil.WriteFile("coll_stamp_up/up"+app.RunFlags.Hostname, []byte("started"), 0666)
	for {
		_, err := os.Stat("coll_stamp_up")
		if err == nil {
			files, _ := ioutil.ReadDir("coll_stamp_up")
			dbg.Lvl4(app.RunFlags.Hostname, "waiting for others to finish", len(files))
			time.Sleep(time.Second)
		} else {
			break
		}
	}
	dbg.Lvl2(app.RunFlags.Hostname, "thinks everybody's here")

	switch app.RunFlags.Mode {
	case "client":
		log.Panic("No client mode")
	case "server":
		RunServer(conf)
	}

}
