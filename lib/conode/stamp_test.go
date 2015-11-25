package conode_test

import (
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"strconv"
	"testing"
)

// Runs two conodes and tests if the value returned is OK
func TestStamp(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()
	go peer1.LoopRounds()
	go peer2.LoopRounds()
	//time.Sleep(time.Second * 2)

	s, err := conode.NewStamp("testdata/config.toml")
	if err != nil {
		t.Fatal("Couldn't open config-file:", err)
	}

	for _, port := range ([]int{2000, 2010}) {
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

	peer1.Close()
	peer2.Close()
}
