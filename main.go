package main
import "flag"
import (
	dbg "github.com/dedis/cothority/helpers/debug_lvl"
	"github.com/dedis/cothority/deploy"
)

var deploy_dst = "deterlab"
var app = ""
var nobuild = false

func init() {
	flag.StringVar(&deploy_dst, "deploy", deploy_dst, "if you want to deploy, chose [deterlab]")
	flag.StringVar(&app, "app", app, "start [server,client] locally")
	flag.IntVar(&dbg.DebugVisible, "debug", dbg.DebugVisible, "Debugging-level. 0 is silent, 5 is flood")
	flag.BoolVar(&nobuild, "nobuild", false, "Don't rebuild all helpers")

	/*
	flag.StringVar(&login, "user", login, "User on the deterlab-machines")
	flag.StringVar(&host, "host", host, "User on the deterlab-machines")
	flag.StringVar(&project, "project", project, "Name of the project on DeterLab")

	flag.BoolVar(&nobuild, "nobuild", false, "Don't rebuild all helpers")
	flag.IntVar(&deploy_config.Nmachs, "machines", deploy_config.Nmachs, "Number of machines (servers running the client)")
	flag.IntVar(&deploy_config.Nloggers, "loggers", deploy_config.Nloggers, "Number of loggers")
	flag.IntVar(&port, "port", port, "Port to forward debugging-information")
	flag.IntVar(&deploy_config.Bf, "branch", deploy_config.Bf, "Branching Factor")
	flag.IntVar(&deploy_config.Hpn, "hpn", deploy_config.Hpn, "Host per node (physical machine)")
	flag.IntVar(&deploy_config.Debug, "debug", deploy_config.Debug, "Debugging-level. 0 is silent, 5 is flood")
*/
}

func main() {
	flag.Parse()

	switch app{
	default:
		switch deploy_dst{
		default:
			dbg.Lvl1("Sorry, not yet implemented")
		case "deterlab":
			dbg.Lvl1("Deploying to deterlab")
			deploy.Start("deterlab", nobuild)
		}
	case "server", "client":
		dbg.Lvl1("Sorry, not yet implemented")
	}
}