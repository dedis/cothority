package conode_test

import (
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sign"
	"strconv"
	"testing"
	"time"
)

// Runs two conodes and tests if the value returned is OK
func TestStampWithoutException(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()
	go peer1.LoopRounds(conode.RoundStamperListenerType, 4)
	go peer2.LoopRounds(conode.RoundStamperListenerType, 4)
	time.Sleep(2 * time.Second)

	s, err := conode.NewStamp("testdata/config.toml")
	if err != nil {
		t.Fatal("Couldn't open config-file:", err)
	}

	for _, port := range []int{7000, 7010} {
		stamper := "localhost:" + strconv.Itoa(port)
		dbg.Lvl2("Contacting stamper", stamper)
		tsm, err := s.GetStamp([]byte("test"), stamper)
		dbg.Lvl3("Evaluating results of", stamper)
		if err != nil {
			t.Fatal("Couldn't get stamp from server:", err)
		}

		if !tsm.AggPublic.Equal(s.X0) {
			t.Fatal("Not correct aggregate public key")
		}
	}

	dbg.Lvl2("Closing peer1")
	peer1.Close()
	dbg.Lvl2("Closing peer2")
	peer2.Close()
	dbg.Lvl2("Closing stamp listeners")
	conode.StampListenersClose()
	dbg.Lvl3("Done with test")
}

func TestStampWithExceptionRaised(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()
	// 2nd node should fail:
	sign.ExceptionForceFailure = peer2.Name()
	go peer1.LoopRounds(conode.RoundStamperListenerType, 4)
	go peer2.LoopRounds(conode.RoundStamperListenerType, 4)
	time.Sleep(2 * time.Second)

	s, err := conode.NewStamp("testdata/config.toml")
	if err != nil {
		t.Fatal("Couldn't open config-file:", err)
	}

	for _, port := range []int{7000, 7010} {
		stamper := "localhost:" + strconv.Itoa(port)
		dbg.Lvl2("Contacting stamper", stamper)
		_, err := s.GetStamp([]byte("test"), stamper)
		dbg.Lvl3("Evaluating results of", stamper)
		if err != nil {
			t.Fatal("Couldn't get stamp from server:", err)
		}

	}

	dbg.Lvl2("Closing peer1")
	peer1.Close()
	dbg.Lvl2("Closing peer2")
	peer2.Close()
	dbg.Lvl2("Closing stamp listeners")
	conode.StampListenersClose()
	dbg.Lvl3("Done with test")
	// reset global var:
	sign.ExceptionForceFailure = ""

}

//// Runs two peers and looks for the exception-message in the stamp
//func TestStampWithExceptionRaised(t *testing.T) {
//dbg.TestOutput(testing.Verbose(), 4)

//peer1, peer2 := createPeers()
//// 2nd node should fail:
//sign.ExceptionForceFailure = peer2.Name()

//// root node:
//dbg.Lvl2(peer1.Name(), "Stamp server in round", 1, "of", 2)
//round, err := sign.NewRoundFromType(conode.RoundStamperListenerType, peer1.Node)
//if err != nil {
//dbg.Fatal("Couldn't create", conode.RoundStamperListenerType, err)
//}
//err = peer1.StartAnnouncement(round)
//if err != nil {
//dbg.Lvl3(err)
//}

//stampClient, err := conode.NewStamp("testdata/config.toml")
//if err != nil {
//t.Fatal("Couldn't open config-file:", err)
//}

//for _, port := range []int{7000, 7010} { // constants from config.toml
//stamper := "localhost:" + strconv.Itoa(port)
//dbg.Lvl2("Contacting stamper", stamper)
//wait := make(chan bool)
//go func() {
//_, err := stampClient.GetStamp([]byte("test"), stamper)
//dbg.Lvl3("Evaluating results of", stamper)
//wait <- true
//if err != nil {
//t.Fatal("Couldn't get stamp from server:", err)
//}
//}()

//dbg.Lvl2(peer1.Name(), "Stamp server in round", 2, "of", 2)
//round, err = sign.NewRoundFromType(conode.RoundStamperListenerType, peer1.Node)
//if err != nil {
//dbg.Fatal("Couldn't create", conode.RoundStamperListenerType, err)
//}
//err = peer1.StartAnnouncement(round)
//if err != nil {
//dbg.Lvl3(err)
//}
//_ = <-wait
//}
//dbg.Lvl2(" TEST will Close All")
//peer1.SendCloseAll()

//dbg.Lvl2("Closing peer1")
//peer1.Close()
//dbg.Lvl2("Closing peer2")
//peer2.Close()
//dbg.Lvl2("Closing stamp listeners")
//conode.StampListenersClose()
//dbg.Lvl3("Done with test")
//// reset global var:
//sign.ExceptionForceFailure = ""
/*}*/
