package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/tree"
	"io/ioutil"
	"os"
	"time"
)

func main() {

	conf := new(app.NTreeConfig)
	app.ReadConfig(conf)

	// we must know who we are
	if app.RunFlags.Hostname == "" {
		dbg.Fatal("Hostname empty: Abort")
	}

	own, depth := findByName(app.RunFlags.Hostname, 0, conf.Tree)
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
		dbg.Panic("No client mode")
	case "server":
		RunServer(conf)
	}

}

func findByName(name string, depth int, t *tree.ConfigTree) (*tree.ConfigTree, int) {
	if name == t.Name {
		return t, depth
	}
	for i := range t.Children {
		c, d := findByName(name, depth+1, t.Children[i])
		if c != nil {
			return c, d
		}
	}
	return nil, depth
}
