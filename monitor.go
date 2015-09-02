package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/browser"
	"golang.org/x/net/websocket"
)

// Monitor monitors log aggregates results into RunStats
func Monitor(bf int) RunStats {
	log.Println("MONITORING")
	defer fmt.Println("DONE MONITORING")
retry_dial:
	ws, err := websocket.Dial("ws://localhost:8080/log", "", "http://localhost/")
	if err != nil {
		time.Sleep(1 * time.Second)
		goto retry_dial
	}
retry:
	// Get HTML of webpage for data (NHosts, Depth, ...)
	doc, err := goquery.NewDocument("http://localhost:8080/")
	if err != nil {
		log.Println("unable to get log data: retrying:", err)
		time.Sleep(10 * time.Second)
		goto retry
	}
	if view {
		browser.OpenURL("http://localhost:8080/")
	}
	nhosts := doc.Find("#numhosts").First().Text()
	log.Println("hosts:", nhosts)
	depth := doc.Find("#depth").First().Text()
	log.Println("depth:", depth)
	nh, err := strconv.Atoi(nhosts)
	if err != nil {
		log.Fatal("unable to convert hosts to be a number:", nhosts)
	}
	d, err := strconv.Atoi(depth)
	if err != nil {
		log.Fatal("unable to convert depth to be a number:", depth)
	}
	clientDone := false
	rootDone := false
	var rs RunStats
	rs.NHosts = nh
	rs.Depth = d
	rs.BF = bf

	var M, S float64
	k := float64(1)
	first := true
	for {
		var data []byte
		err := websocket.Message.Receive(ws, &data)
		if err != nil {
			// if it is an eof error than stop reading
			if err == io.EOF {
				log.Println("websocket terminated before emitting EOF or terminating string")
				break
			}
			continue
		}
		if bytes.Contains(data, []byte("EOF")) || bytes.Contains(data, []byte("terminating")) {
			log.Printf(
				"EOF/terminating Detected: need forkexec to report and clients: rootDone(%t) clientDone(%t)",
				rootDone, clientDone)
		}
		if bytes.Contains(data, []byte("root_round")) {
			if clientDone || rootDone {
				// ignore after we have received our first EOF
				continue
			}
			var entry StatsEntry
			err := json.Unmarshal(data, &entry)
			if err != nil {
				log.Fatal("json unmarshalled improperly:", err)
			}
			log.Println("root_round:", entry)
			if first {
				first = false
				rs.MinTime = entry.Time
				rs.MaxTime = entry.Time
			}
			if entry.Time < rs.MinTime {
				rs.MinTime = entry.Time
			} else if entry.Time > rs.MaxTime {
				rs.MaxTime = entry.Time
			}

			rs.AvgTime = ((rs.AvgTime * (k - 1)) + entry.Time) / k

			var tM = M
			M += (entry.Time - tM) / k
			S += (entry.Time - tM) * (entry.Time - M)
			k++
			rs.StdDev = math.Sqrt(S / (k - 1))
		} else if bytes.Contains(data, []byte("forkexec")) {
			if rootDone {
				continue
			}
			var ss SysStats
			err := json.Unmarshal(data, &ss)
			if err != nil {
				log.Fatal("unable to unmarshal forkexec:", ss)
			}
			rs.SysTime = ss.SysTime
			rs.UserTime = ss.UserTime
			log.Println("FORKEXEC:", ss)
			if clientDone {
				break
			}
			rootDone = true
		} else if bytes.Contains(data, []byte("client_msg_stats")) {
			if clientDone {
				continue
			}
			var cms ClientMsgStats
			err := json.Unmarshal(data, &cms)
			if err != nil {
				log.Fatal("unable to unmarshal client_msg_stats:", string(data))
			}
			// what do I want to keep out of the Client Message States
			// cms.Buckets stores how many were processed at time T
			// cms.RoundsAfter stores how many rounds delayed it was
			//
			// get the average delay (roundsAfter), max and min
			// get the total number of messages timestamped
			// get the average number of messages timestamped per second?
			avg, _, _, _ := ArrStats(cms.Buckets)
			// get the observed rate of processed messages
			// avg is how many messages per second, we want how many milliseconds between messages
			observed := avg / 1000 // set avg to messages per milliseconds
			observed = 1 / observed
			rs.Rate = observed
			rs.Times = cms.Times
			if rootDone {
				break
			}
			clientDone = true
		}
	}
	return rs
}
