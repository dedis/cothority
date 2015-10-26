package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"golang.org/x/net/websocket"
)

// Monitor monitors log aggregates results into RunStats
func Monitor(stats Stats) {
	dbg.Lvl1("Starting monitoring")
	defer dbg.Lvl1("Done monitoring")
retry_dial:
	ws, err := websocket.Dial(fmt.Sprintf("ws://localhost:%d/log", port), "", "http://localhost/")
	if err != nil {
		time.Sleep(1 * time.Second)
		dbg.Lvl2("Can not connect to websocket. Retrying...")
		goto retry_dial
	}
	for {
		var data []byte
		err := websocket.Message.Receive(ws, &data)
		if err != nil {
			// if it is an eof error than stop reading
			if err == io.EOF {
				dbg.Lvl1("websocket terminated before emitting EOF or terminating string")
				break
			}
			continue
		}
		dbg.Lvl3("Received msg", string(data))
		if bytes.Contains(data, []byte("EOF")) || bytes.Contains(data, []byte("terminating")) {
			dbg.Lvl2("EOF/terminating Detected: need forkexec to report.")
		}
		if bytes.Contains(data, []byte("root_round")) {
			dbg.Lvl2("root_round msg received")

			var entry CollServerEntry
			err := json.Unmarshal(data, &entry)
			if err != nil {
				log.Fatal("json unmarshalled improperly:", err)
			}
			if entry.Type != "root_round" {
				dbg.Lvl1("Wrong debugging message - ignoring")
				continue
			}
			dbg.Lvl4("root_round:", entry)
			stats.AddEntry(entry)
		} else if bytes.Contains(data, []byte("basic_")) {
			var entry BasicEntry
			err := json.Unmarshal(data, &entry)
			if err != nil {
				log.Fatal("json unmarshalled improperly:", err)
			}
			stats.AddEntry(entry)
			dbg.Lvl2("Basic entry:", entry)
		} else if bytes.Contains(data, []byte("end")) {
			dbg.Lvl2("Received end")
			break
		}
	}
}
