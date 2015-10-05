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

	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/websocket"
)

// Monitor monitors log aggregates results into RunStats
func Monitor() RunStats {
	if platform_dst != "deterlab"{
		dbg.Lvl1("Not starting monitor as not in deterlab-mode!")
		return RunStats{}
	}
	dbg.Lvl1("Starting monitoring")
	defer dbg.Lvl1("Done monitoring")
	retry_dial:
	ws, err := websocket.Dial(fmt.Sprintf("ws://localhost:%d/log", port), "", "http://localhost/")
	if err != nil {
		time.Sleep(1 * time.Second)
		goto retry_dial
	}
	retry:
	// Get HTML of webpage for data (NHosts, Depth, ...)
	doc, err := goquery.NewDocument(fmt.Sprintf("http://localhost:%d/", port))
	if err != nil {
		dbg.Lvl4("unable to get log data: retrying:", err)
		time.Sleep(10 * time.Second)
		goto retry
	}
	nhosts_str := doc.Find("#numhosts").First().Text()
	dbg.Lvl3("hosts:", nhosts_str)
	depth_str := doc.Find("#depth").First().Text()
	dbg.Lvl3("depth:", depth_str)
	bf_str := doc.Find("#bf").First().Text()
	dbg.Lvl3("bf:", bf_str)
	nh, err := strconv.Atoi(nhosts_str)
	if err != nil {
		log.Fatal("unable to convert hosts to be a number:", nhosts_str)
	}
	d, err := strconv.Atoi(depth_str)
	if err != nil {
		log.Fatal("unable to convert depth to be a number:", depth_str)
	}
	bf, err := strconv.Atoi(bf_str)
	if err != nil {
		log.Fatal("unable to convert bf to be a number:", bf_str)
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
				dbg.Lvl4("websocket terminated before emitting EOF or terminating string")
				break
			}
			continue
		}
		dbg.Lvl5("Received msg", data)
		if bytes.Contains(data, []byte("EOF")) || bytes.Contains(data, []byte("terminating")) {
			dbg.Lvl2(
				"EOF/terminating Detected: need forkexec to report and clients: rootDone", rootDone, "clientDone", clientDone)
		}
		if bytes.Contains(data, []byte("root_round")) {
			dbg.Lvl3("root_round msg received (clientDone = ", clientDone, ", rootDone = ", rootDone, ")")

			if clientDone || rootDone {
				dbg.Lvl4("Continuing searching data")
				// ignore after we have received our first EOF
				continue
			}
			var entry StatsEntry
			err := json.Unmarshal(data, &entry)
			if err != nil {
				log.Fatal("json unmarshalled improperly:", err)
			}
			if entry.Type != "root_round" {
				dbg.Lvl1("Wrong debugging message - ignoring")
				continue
			}
			dbg.Lvl4("root_round:", entry)
			if entry.Round == 1{
				dbg.Lvl1("Throwing away first round")
				continue
			}
			if first {
				first = false
				dbg.Lvl4("Setting min-time to", entry.Time)
				rs.MinTime = entry.Time
				rs.MaxTime = entry.Time
			}
			if entry.Time < rs.MinTime {
				dbg.Lvl4("Setting min-time to", entry.Time)
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
		} else if bytes.Contains(data, []byte("schnorr_round")) {

			var entry StatsEntry
			err := json.Unmarshal(data, &entry)
			if err != nil {
				log.Fatal("json unmarshalled improperly:", err)
			}
			if entry.Type != "schnorr_round" {
				dbg.Lvl1("Wrong debugging message - ignoring")
				continue
			}
			dbg.Lvl4("schnorr_round:", entry)
			if first {
				first = false
				dbg.Lvl4("Setting min-time to", entry.Time)
				rs.MinTime = entry.Time
				rs.MaxTime = entry.Time
			}
			if entry.Time < rs.MinTime {
				dbg.Lvl4("Setting min-time to", entry.Time)
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
		} else if bytes.Contains(data, []byte("schnorr_end")){
			break
		} else if bytes.Contains(data, []byte("forkexec")) {
			dbg.Lvl3("Received forkexec")
			if rootDone {
				dbg.Lvl2("RootDone is true - continuing")
				continue
			}
			var ss SysStats
			err := json.Unmarshal(data, &ss)
			if err != nil {
				log.Fatal("unable to unmarshal forkexec:", ss)
			}
			rs.SysTime = ss.SysTime
			rs.UserTime = ss.UserTime
			dbg.Lvl4("forkexec:", ss)
			rootDone = true
			dbg.Lvl2("Forkexec msg received (clientDone = ", clientDone, ", rootDone = ", rootDone, ")")
			if clientDone {
				break
			}
		} else if bytes.Contains(data, []byte("client_msg_stats")) {
			if clientDone {
				dbg.Lvl2("Continuing because client is already done")
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
			dbg.Lvl2("Client Msg stats received (clientDone = ", clientDone, ",rootDone = ", rootDone, ")")
			clientDone = true
			if rootDone {
				break
			}
		}
	}
	return rs
}
