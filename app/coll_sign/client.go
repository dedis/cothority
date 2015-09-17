package coll_sign
import (
	"github.com/dedis/cothority/deploy"
	"time"
	"github.com/dedis/cothority/lib/logutils"
	"github.com/dedis/cothority/proto/sign"
	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/oldconfig"
)

func RunClient(conf *deploy.Config, hc *oldconfig.HostConfig) {
	if hc.SNodes[0].IsRoot(0) {
		time.Sleep(3 * time.Second)
		start := time.Now()
		iters := 10

		for i := 0; i < iters; i++ {
			//time.Sleep(3 * time.Second)
			start = time.Now()
			//fmt.Println("ANNOUNCING")
			hc.SNodes[0].LogTest = []byte("Hello World")
			dbg.Lvl2("Going to launch announcement ", hc.SNodes[0].Name())
			err := hc.SNodes[0].Announce(0,
				&sign.AnnouncementMessage{
					LogTest: hc.SNodes[0].LogTest,
					Round:   i})
			if err != nil {
				dbg.Lvl1(err)
			}
			elapsed := time.Since(start)
			log.WithFields(log.Fields{
				"file":  logutils.File(),
				"type":  "root_announce",
				"round": i,
				"time":  elapsed,
			}).Info("")
		}

	} else {
		// otherwise wait a little bit (hopefully it finishes by the end of this)
		time.Sleep(30 * time.Second)
	}
}
