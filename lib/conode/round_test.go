package conode_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"testing"
	"time"
	"github.com/dedis/cothority/lib/sign"
)

// Tests if the rounds are deleted when done
func TestDeleteRounds(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()

	if len(peer1.Rounds) != 0 {
		t.Fatal("There should be 0 rounds to start with")
	}

	round, err := sign.NewRoundFromType("cosistamper", peer1.Node)
	if err != nil {
		t.Fatal("Couldn't create cosi-round")
	}

	peer1.StartAnnouncement(round)
	if len(peer1.Rounds) != 1 {
		t.Fatal("Created one round - should be there")
	}

	time.Sleep(time.Second)

	if len(peer1.Rounds) != 0 {
		t.Fatal("Doing one round shouldn't take more than 1 second")
	}

	peer1.Close()
	peer2.Close()
}

func TestRoundCosi(t *testing.T){
	testRound(t, "cosi")
}

func TestRoundStamper(t *testing.T){
	testRound(t, "stamper")
}

func TestRoundCosiStamper(t *testing.T){
	testRound(t, "cosistamper")
}

// For testing the different round-types
// Every round-type is in his own Test*-method,
// so one can easily run just a given round-test
func testRound(t *testing.T, roundType string) {
	dbg.TestOutput(testing.Verbose(), 4)
	dbg.Lvl2("Testing", roundType)
	peer1, peer2 := createPeers()

	round, err := sign.NewRoundFromType(roundType, peer1.Node)
	if err != nil {
		t.Fatal("Couldn't create cosi-round:", err)
	}

	peer1.StartAnnouncement(round)

	peer1.Close()
	peer2.Close()
}
