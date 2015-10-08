package main
import (
	"time"
	"github.com/dedis/cothority/lib/logutils"
	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"sync/atomic"

	"strconv"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/graphs"
)

var MAX_N_SECONDS int = 1 * 60 * 60 // 1 hours' worth of seconds
var MAX_N_ROUNDS int = MAX_N_SECONDS / int(ROUND_TIME / time.Second)
var ROUND_TIME time.Duration = 1 * time.Second

var done = make(chan string, 1)

func RunClient(conf *app.ConfigColl, hc *graphs.HostConfig) {
	buck := make([]int64, 300)
	roundsAfter := make([]int64, MAX_N_ROUNDS)
	times := make([]int64, MAX_N_SECONDS * 1000) // maximum number of milliseconds (maximum rate > 1 per millisecond)

	dbg.Lvl1("Going to run client and asking servers to print")
	time.Sleep(3 * time.Second)
	hc.SNodes[0].RegisterDoneFunc(RoundDone)
	start := time.Now()
	tFirst := time.Now()

	for i := 0; i < conf.Rounds; i++ {
		time.Sleep(time.Second)
		hc.SNodes[0].LogTest = []byte("Hello World")
		dbg.Lvl3("Going to launch announcement ", hc.SNodes[0].Name())
		start = time.Now()
		t0 := time.Now()
		sys, usr := app.GetRTime()

		err := hc.SNodes[0].StartSigningRound()
		if err != nil {
			dbg.Lvl1(err)
		}

		select {
		case msg := <-done:
			dbg.Lvl3("Received reply from children", msg)
		case <-time.After(10 * ROUND_TIME):
			dbg.Fatal("client timeouted on waiting for response")
			continue
		}

		t := time.Since(t0)
		elapsed := time.Since(start)
		secToTimeStamp := t.Seconds()
		secSinceFirst := time.Since(tFirst).Seconds()
		atomic.AddInt64(&buck[int(secSinceFirst)], 1)
		index := int(secToTimeStamp) / int(ROUND_TIME / time.Second)
		atomic.AddInt64(&roundsAfter[index], 1)
		atomic.AddInt64(&times[i], t.Nanoseconds())
		log.WithFields(log.Fields{
			"file":  logutils.File(),
			"type":  "root_announce",
			"round": i,
			"time":  elapsed,
		}).Info("")

		dSys, dUsr := app.GetDiffRTime(sys, usr)
		log.WithFields(log.Fields{
			"file":  logutils.File(),
			"type":  "root_round",
			"round": i,
			"time":  dSys + dUsr,
		}).Info("root round")
	}

	log.WithFields(log.Fields{
		"file":        logutils.File(),
		"type":        "client_msg_stats",
		"buck":        removeTrailingZeroes(buck),
		"roundsAfter": removeTrailingZeroes(roundsAfter),
		"times":       removeTrailingZeroes(times),
	}).Info("")

	// And tell everybody to quit
	err := hc.SNodes[0].CloseAll(hc.SNodes[0].Round)
	if err != nil {
		log.Fatal("Couldn't close:", err)
	}
}

func RoundDone(view int, SNRoot hashid.HashId, LogHash hashid.HashId, p proof.Proof) {
	dbg.Lvl3(view, "finished round")
	done <- "Done with view: " + strconv.Itoa(view)
}

func removeTrailingZeroes(a []int64) []int64 {
	i := len(a) - 1
	for ; i >= 0; i-- {
		if a[i] != 0 {
			break
		}
	}
	return a[:i + 1]
}

