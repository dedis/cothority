package main

import (
	"crypto/rand"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/hashid"
)

var muStats sync.Mutex

var MAX_N_SECONDS int = 1 * 60 * 60 // 1 hours' worth of seconds
var MAX_N_ROUNDS int = MAX_N_SECONDS / int(ROUND_TIME / time.Second)

func RunClient(flags *app.Flags, conf *app.ConfigColl) {
	dbg.Lvl4("Starting to run stampclient")
	c := NewClient(flags.Name)
	servers := strings.Split(flags.Server, ",")
	// take the right percentage of servers
	servers = scaleServers(flags, conf, servers)
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
	streamMessgs(c, servers, conf.Rate)
	dbg.Lvl4("Finished streaming", flags.Name)
	return
}

// scaleServers will take the right percentage of server to contact to stamp
// request. If percentage is 0, only contact the leader (if the client is on the
// same physical machine than the leader/root).
func scaleServers(flags *app.Flags, conf *app.ConfigColl, servers []string) []string {
	if len(servers) == 0 || conf.StampRatio > 1 {
		dbg.Lvl1("Client wont change the servers percentage ")
		return servers
	}
	if conf.StampRatio == -1 {
		// take only the root if  we are a "root client" also
		if flags.AmRoot {
			dbg.Lvl1("Client will only contact root")
			return []string{servers[0]}
		} else {
			// others client dont do nothing
			dbg.Lvl3("Client wont contact anyone")
			return []string{}
		}
	}
	// else take the right perc
	i := int(math.Ceil(conf.StampRatio * float64(len(servers))))
	fn := dbg.Lvl3
	if flags.AmRoot {
		fn = dbg.Lvl1
	}
	fn("Client will contact", i, "/", len(servers), "servers")
	return servers[0:i]
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

func streamMessgs(c *Client, servers []string, rate int) {
<<<<<<< HEAD
	dbg.Lvl4(c.Name(), "streaming at given rate", rate, "msgs per seconds")
=======
	nServers := len(servers)
	if nServers == 0 {
		dbg.Lvl3("Stamp Client wont stream messages")
		return
	}
>>>>>>> development
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	dbg.Lvl1(c.Name(), "streaming at given rate", rate, " msg / s")
	msg := genRandomMessages(1)[0]
	i := 0

	retry:
	dbg.Lvl3(c.Name(), "checking if", servers[0], "is already up")
	err := c.TimeStamp(msg, servers[0])
	if err == io.EOF || err == coconet.ErrClosed {
		dbg.Lvl4("Client", c.Name(), "Couldn't connect to TimeStamp")
		return
	} else if err == ErrClientToTSTimeout {
		dbg.Lvl4(err.Error())
	} else if err != nil {
		time.Sleep(500 * time.Millisecond)
		goto retry
	}
	dbg.Lvl3(c.Name(), "successfully connected to", servers[0])

	// every tick send a time coll_stamp request to every server specified
	// this will stream until we get an EOF
	tick := 0
	abort := false
	for _ = range ticker.C {
		tick += 1
		go func(msg []byte, s string, tick int) {
			dbg.Lvl4("StampClient will try stamprequest")
			err := c.TimeStamp(msg, s)

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

		}(msg, servers[i], tick)

		i = (i + 1) % nServers
		if abort {
			break
		}
		if ( tick % 5000 ) == 0 {
			dbg.Lvl3("Sent", tick, "timestamps so far to", nServers, "servers")
		}

	}

	return
}
