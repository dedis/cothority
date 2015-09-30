package deploy

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
	clientDone := false
	rootDone := false
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
		if bytes.Contains(data, []byte("EOF")) || bytes.Contains(data, []byte("terminating")) {
			dbg.Lvl2(
				"EOF/terminating Detected: need forkexec to report and clients: rootDone", rootDone, "clientDone", clientDone)
		}
		if bytes.Contains(data, []byte("root_round")) {
			dbg.Lvl4("root_round msg received (clientDone = ", clientDone, ", rootDone = ", rootDone, ")")

			if clientDone || rootDone {
				dbg.Lvl4("Continuing searching data")
				// ignore after we have received our first EOF
				continue
			}
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
		} else if bytes.Contains(data, []byte(ShamirRoundType)) {

			var entry ShamirEntry
			err := json.Unmarshal(data, &entry)
			if err != nil {
				log.Fatal("json unmarshalled improperly:", err)
			}
			stats.AddEntry(entry)
			dbg.Lvl2("Monitor() : received shamir_round:", entry)
		} else if bytes.Contains(data, []byte(ShamirSetupType)) {
			var entry ShamirEntry
			err := json.Unmarshal(data, &entry)
			if err != nil {
				log.Fatal("json unmarshalled improperly:", err)
			}
			dbg.Lvl2("shamir_setup entry:", entry)
			stats.AddEntry(entry)
		} else if bytes.Contains(data, []byte("schnorr_end")) {
			break
		} else if bytes.Contains(data, []byte("forkexec")) {
			if rootDone {
				continue
			}
			var ss SysEntry
			err := json.Unmarshal(data, &ss)
			if err != nil {
				log.Fatal("unable to unmarshal forkexec:", ss)
			}
			stats.AddEntry(ss)
			rootDone = true
			dbg.Lvl2("Monitor() Forkexec msg received (clientDone = ", clientDone, ", rootDone = ", rootDone, ")")
			if clientDone {
				break
			}
		} else if bytes.Contains(data, []byte("client_msg_stats")) {
			if clientDone {
				continue
			}
			var cms CollClientEntry
			err := json.Unmarshal(data, &cms)
			if err != nil {
				log.Fatal("unable to unmarshal client_msg_stats:", string(data))
			}
			stats.AddEntry(cms)
			dbg.Lvl2("Monitor() Client Msg stats received (clientDone = ", clientDone, ",rootDone = ", rootDone, ")")
			clientDone = true
			if rootDone {
				break
			}
		}
	}
}
