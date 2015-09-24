// Cothority - framework for co-authority based research
//
//

package main
import (
	"flag"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
)

var deploy_dst = "deterlab"
var app = ""
var nobuild = false
var build = ""
var machines = 3

func init() {
	flag.StringVar(&deploy_dst, "deploy", deploy_dst, "if you want to deploy, chose [deterlab]")
	flag.StringVar(&app, "app", app, "start [server,client] locally")
	flag.IntVar(&dbg.DebugVisible, "debug", dbg.DebugVisible, "Debugging-level. 0 is silent, 5 is flood")
	flag.BoolVar(&nobuild, "nobuild", false, "Don't rebuild all helpers")
	flag.StringVar(&build, "build", "", "List of packages to build")
	flag.IntVar(&machines, "machines", machines, "Number of machines on Deterlab")
}

func main() {
	flag.Parse()

	switch app{
	default:
		switch deploy_dst{
		default:
			dbg.Lvl1("Sorry, deployment method", deploy_dst, "not yet implemented")
		case "deterlab":
			dbg.Lvl1("Deploying to deterlab")
			Start("deterlab")
		}
	case "server", "client":
		dbg.Lvl1("Sorry,", app, "not yet implemented")
	}
}