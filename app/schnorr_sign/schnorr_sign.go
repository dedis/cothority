package schnorr_sign

import (
	"github.com/dedis/cothority/deploy"
	"github.com/dedis/cothority/lib/config"
	//dbg "github.com/dedis/cothority/lib/debug_lvl"
	//"os"
)

// Dispatch-function for running either client or server (mode-parameter)
func Run(app *config.AppConfig, conf *deploy.Config) {

	// we must know who we are
	//if app.Hostname == "" {
	//	dbg.Lvl1("Hostname empty : Abort")
	//	os.Exit(1)
	//}

	//dbg.Lvl1(app.Hostname, "Starting to run as ", app.Mode)
	//var err error
	//s := GetSuite(conf.Suite)
	//opts := config.ConfigOptions{ConnType: "tcp", Host: app.Hostname, Suite: s, NoTree: true}
	//hc, err = config.LoadConfig("tree.json", opts)
	//// from here on, hc.SNodes contains all the nodes = all the connections to all peers ?
	//// Do some common setup
	//switch app.Mode {
	//case "client":
	//	RunClient(conf)
	//case "server":
	//	RunServer(conf)
	//}
}
