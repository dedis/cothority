package stampclient

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
	dbg "github.com/ineiti/cothorities/helpers/debug_lvl"

	"github.com/ineiti/cothorities/helpers/coconet"
	"github.com/ineiti/cothorities/helpers/hashid"
	"github.com/ineiti/cothorities/helpers/logutils"
	"github.com/ineiti/cothorities/application/stamp"
)

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

var muStats sync.Mutex

func AggregateStats(buck, roundsAfter, times []int64) string {
	return "Client Finished Aggregating Statistics"
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

func streamMessgs(c *stamp.Client, servers []string, rate int) {
	dbg.Lvl3("STREAMING: GIVEN RATE", rate)
	// buck[i] = # of timestamp responses received in second i
	buck := make([]int64, MAX_N_SECONDS)
	// roundsAfter[i] = # of timestamp requests that were processed i rounds late
	roundsAfter := make([]int64, MAX_N_ROUNDS)
	times := make([]int64, MAX_N_SECONDS * 1000) // maximum number of milliseconds (maximum rate > 1 per millisecond)
	ticker := time.Tick(time.Duration(rate) * time.Millisecond)
	msg := genRandomMessages(1)[0]
	i := 0
	nServers := len(servers)

	retry:
	err := c.TimeStamp(msg, servers[0])
	if err == io.EOF || err == coconet.ErrClosed {
		dbg.Lvl3("Client", c.Name(), "DONE: couldn't connect to TimeStamp")
		log.Fatal(AggregateStats(buck, roundsAfter, times))
	} else if err == stamp.ErrClientToTSTimeout {
		log.Errorln(err)
	} else if err != nil {
		time.Sleep(500 * time.Millisecond)
		goto retry
	}

	tFirst := time.Now()

	// every tick send a time stamp request to every server specified
	// this will stream until we get an EOF
	tick := 0
	for _ = range ticker {
		tick += 1
		go func(msg []byte, s string, tick int) {
			t0 := time.Now()
			err := c.TimeStamp(msg, s)
			t := time.Since(t0)

			if err == io.EOF || err == coconet.ErrClosed {
				if err == io.EOF {
					dbg.Lvl3("CLIENT ", c.Name(), "DONE: terminating due to EOF", s)
				} else {
					dbg.Lvl3("CLIENT ", c.Name(), "DONE: terminating due to Connection Error Closed", s)
				}
				log.Fatal(AggregateStats(buck, roundsAfter, times))
			} else if err != nil {
				// ignore errors
				dbg.Lvl3("CLIENT ", c.Name(), "Leaving out streamMessages. ", err)
				return
			}

			// TODO: we might want to subtract a buffer from secToTimeStamp
			// to account for computation time
			secToTimeStamp := t.Seconds()
			secSinceFirst := time.Since(tFirst).Seconds()
			atomic.AddInt64(&buck[int(secSinceFirst)], 1)
			index := int(secToTimeStamp) / int(stamp.ROUND_TIME / time.Second)
			atomic.AddInt64(&roundsAfter[index], 1)
			atomic.AddInt64(&times[tick], t.Nanoseconds())

		}(msg, servers[i], tick)

		i = (i + 1) % nServers
	}

}

var MAX_N_SECONDS int = 1 * 60 * 60 // 1 hours' worth of seconds
var MAX_N_ROUNDS int = MAX_N_SECONDS / int(stamp.ROUND_TIME / time.Second)

func Run(server string, nmsgs int, name string, rate int, debug int) {
	dbg.Lvl3("Starting to run stampclient")
	c := stamp.NewClient(name)
	servers := strings.Split(server, ",")

	// connect to all the servers listed
	for _, s := range servers {
		h, p, err := net.SplitHostPort(s)
		if err != nil {
			log.Fatal("improperly formatted host")
		}
		pn, _ := strconv.Atoi(p)
		c.AddServer(s, coconet.NewTCPConn(net.JoinHostPort(h, strconv.Itoa(pn + 1))))
	}

	// Check if somebody asks for the old way
	if rate < 0 {
		log.Fatal("ROUNDS BASED RATE LIMITING DEPRECATED")
	}

	// Stream time stamp requests
	// if rate specified send out one message every rate milliseconds
	dbg.Lvl1(name, "starting to stream at rate", rate)
	streamMessgs(c, servers, rate)
	dbg.Lvl3("Finished streaming")
	return

	/*
	msgs := genRandomMessages(nmsgs + 20)
	// rounds based messaging
	r := 0
	s := 0

	// ROUNDS BASED IS DEPRECATED
	log.Fatal("ROUNDS BASED RATE LIMITING DEPRECATED")
	for {
		//start := time.Now()
		var wg sync.WaitGroup
		for i := 0; i < nmsgs; i++ {
			wg.Add(1)
			go func(i, s int) {
				defer wg.Done()
				err := c.TimeStamp(msgs[i], servers[s])
				if err == io.EOF {
					log.WithFields(log.Fields{
						"file":        logutils.File(),
						"type":        "client_msg_stats",
						"buck":        make([]int64, 0),
						"roundsAfter": make([]int64, 0),
					}).Info("")

					log.Fatal("EOF: terminating time client")
				}
			}(i, s)
			s = (s + 1) % len(servers)
		}
		wg.Wait()
		//elapsed := time.Since(start)
		dbg.Lvl3("client done with round")
		//log.WithFields(log.Fields{
		//"file":  logutils.File(),
		//"type":  "client_round",
		//"round": r,
		//"time":  elapsed,
		//}).Info("client round")
		r++
	}
	*/
}
