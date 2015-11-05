package main

import (
	"crypto/rand"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/logutils"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/proto/sign"
)

var muStats sync.Mutex

var MAX_N_SECONDS int = 1 * 60 * 60 // 1 hours' worth of seconds
var MAX_N_ROUNDS int = MAX_N_SECONDS / int(sign.ROUND_TIME / time.Second)

func RunClient(flags *app.Flags, conf *app.ConfigColl) {
	dbg.Lvl4("Starting to run stampclient")
	c := NewClient(flags.Name)
	servers := strings.Split(flags.Server, ",")

	// connect to all the servers listed
	for _, s := range servers {
		h, p, err := net.SplitHostPort(s)
		if err != nil {
			log.Fatal("improperly formatted host")
		}
		pn, _ := strconv.Atoi(p)
		c.AddServer(s, coconet.NewTCPConn(net.JoinHostPort(h, strconv.Itoa(pn + 1))))
	}

	// Stream time coll_stamp requests
	// if rate specified send out one message every rate milliseconds
	dbg.Lvl3(flags.Name, "starting to stream at rate", conf.Rate, "with root", flags.AmRoot)
	buck, roundsAfter, times := streamMessgs(c, servers, conf.Rate)
	if flags.AmRoot {
		dbg.Lvl3("Printing stats")
		AggregateStats(buck, roundsAfter, times)
	}
	dbg.Lvl4("Finished streaming", flags.Name)
	return
}

func AggregateStats(buck, roundsAfter, times []int64) {
	muStats.Lock()
	log.WithFields(log.Fields{
		"file":        logutils.File(),
		"type":        "client_msg_stats",
		"buck":        removeTrailingZeroes(buck),
		"roundsAfter": removeTrailingZeroes(roundsAfter),
		"times":       removeTrailingZeroes(times),
	}).Info("")
	muStats.Unlock()
}

func genRandomMessages(n int) [][]byte {
	msgs := make([][]byte, n)
	for i := range msgs {
		msgs[i] = make([]byte, hashid.Size)
		_, err := rand.Read(msgs[i])
		if err != nil {
			log.Fatal("failed to generate random commit:", err)
		}
	}
	return msgs
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

func streamMessgs(c *Client, servers []string, rate int) ([]int64, []int64, []int64) {
	dbg.Lvl4(c.Name(), "streaming at given rate", rate)
	// buck[i] = # of timestamp responses received in second i
	buck := make([]int64, MAX_N_SECONDS)
	// roundsAfter[i] = # of timestamp requests that were processed i rounds late
	roundsAfter := make([]int64, MAX_N_ROUNDS)
	times := make([]int64, MAX_N_SECONDS * 1000) // maximum number of milliseconds (maximum rate > 1 per millisecond)
	ticker := time.NewTicker(time.Duration(rate) * time.Millisecond)
	msg := genRandomMessages(1)[0]
	i := 0
	nServers := len(servers)

	retry:
	dbg.Lvl3(c.Name(), "checking if", servers[0], "is already up")
	err := c.TimeStamp(msg, servers[0])
	if err == io.EOF || err == coconet.ErrClosed {
		dbg.Lvl4("Client", c.Name(), "Couldn't connect to TimeStamp")
		return buck, roundsAfter, times
	} else if err == ErrClientToTSTimeout {
		dbg.Lvl4(err.Error())
	} else if err != nil {
		time.Sleep(500 * time.Millisecond)
		goto retry
	}
	dbg.Lvl3(c.Name(), "successfully connected to", servers[0])

	tFirst := time.Now()

	// every tick send a time coll_stamp request to every server specified
	// this will stream until we get an EOF
	tick := 0
	abort := false
	for _ = range ticker.C {
		tick += 1
		go func(msg []byte, s string, tick int) {
			t0 := time.Now()
			err := c.TimeStamp(msg, s)
			t := time.Since(t0)

			if err == io.EOF || err == coconet.ErrClosed {
				if err == io.EOF {
					dbg.Lvl4("Client", c.Name(), "terminating due to EOF", s)
				} else {
					dbg.Lvl4("Client", c.Name(), "terminating due to Connection Error Closed", s)
				}
				abort = true
				return
			} else if err != nil {
				// ignore errors
				dbg.Lvl4("Client", c.Name(), "Leaving out streamMessages. ", err)
				return
			}

			// TODO: we might want to subtract a buffer from secToTimeStamp
			// to account for computation time
			secToTimeStamp := t.Seconds()
			secSinceFirst := time.Since(tFirst).Seconds()
			atomic.AddInt64(&buck[int(secSinceFirst)], 1)
			index := int(secToTimeStamp) / int(sign.ROUND_TIME / time.Second)
			atomic.AddInt64(&roundsAfter[index], 1)
			atomic.AddInt64(&times[tick], t.Nanoseconds())

		}(msg, servers[i], tick)

		i = (i + 1) % nServers
		if abort {
			break
		}
	}

	return buck, roundsAfter, times
}
