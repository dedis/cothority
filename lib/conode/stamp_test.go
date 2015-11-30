package conode_test

import (
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"strconv"
	"testing"
"time"
)

// Runs two conodes and tests if the value returned is OK
func TestStamp(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()
	go peer1.LoopRounds(conode.RoundStamperListenerType, 2)
	go peer2.LoopRounds(conode.RoundStamperListenerType, 2)
	time.Sleep(time.Second)

	s, err := conode.NewStamp("testdata/config.toml")
	if err != nil {
		t.Fatal("Couldn't open config-file:", err)
	}

	for _, port := range ([]int{7000, 7010}) {
		stamper := "localhost:" + strconv.Itoa(port)
		dbg.Lvl2("Contacting stamper", stamper)
		tsm, err := s.GetStamp([]byte("test"), stamper)
		dbg.Lvl3("Evaluating results of", stamper)
		if err != nil {
			t.Fatal("Couldn't get stamp from server:", err)
		}

		if !tsm.Srep.AggPublic.Equal(s.X0) {
			t.Fatal("Not correct aggregate public key")
		}
	}

	dbg.Lvl2("Closing peer1")
	peer1.Close()
	dbg.Lvl2("Closing peer2")
	peer2.Close()
	dbg.Lvl3("Done with test")
}
