package coll_sign
import
(
	"github.com/dedis/cothority/deploy"
	"time"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/config"
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/proto/sign"
)

func RunServer(app *config.AppConfig, conf *deploy.Config, hc *config.HostConfig) {
	// run this specific host
	err := hc.Run(false, sign.MerkleTree, app.Hostname)
	if err != nil {
		log.Fatal(err)
	}

	dbg.Lvl1(app.Hostname, "started up in server-mode")

	// Let's start the client if we're the root-node
	if hc.SNodes[0].IsRoot(0) {
		dbg.Lvl1(app.Hostname, "started client")
		RunClient(conf, hc)
	} else{
		// Endless-loop till we stop by tearing down the connections
		for {
			time.Sleep(time.Minute)
		}
	}
}
