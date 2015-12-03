package conode_test

import (
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sign"
	"testing"
	"time"
)

// Tests if the rounds are deleted when done
func TestDeleteRounds(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()

	if len(peer1.Rounds) != 0 {
		t.Fatal("There should be 0 rounds to start with")
	}

	round, err := sign.NewRoundFromType(conode.RoundStamperListenerType, peer1.Node)
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

func TestRoundException(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()
	sign.ExceptionForceFailure = peer2.Name()

	round, err := sign.NewRoundFromType(sign.RoundExceptionType, peer1.Node)
	if err != nil {
		t.Fatal("Couldn't create Exception round:", err)
	}

	peer1.StartAnnouncement(round)
	time.Sleep(time.Second)

	cosi := round.(*sign.RoundException).Cosi
	if cosi.R_hat == nil {
		t.Fatal("Didn't finish round - R_hat empty")
	}
	err = cosi.VerifyResponses()
	if err != nil {
		t.Fatal("Couldn't verify responses")
	}
	peer1.Close()
	peer2.Close()
}

func TestRoundCosi(t *testing.T) {
	testRound(t, sign.RoundCosiType)
}

func TestRoundStamper(t *testing.T) {
	testRound(t, conode.RoundStamperType)
}

func TestRoundCosiStamper(t *testing.T) {
	testRound(t, conode.RoundStamperListenerType)
}

func TestRoundSetup(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	roundType := "setup"
	dbg.Lvl2("Testing", roundType)
	peer1, peer2 := createPeers()

	round, err := sign.NewRoundFromType(roundType, peer1.Node)
	if err != nil {
		t.Fatal("Couldn't create", roundType, "round:", err)
	}

	peer1.StartAnnouncement(round)
	time.Sleep(time.Second)

	counted := <-round.(*sign.RoundSetup).Counted
	if counted != 2 {
		t.Fatal("Counted", counted, "nodes, but should be 2")
	}

	peer1.Close()
	peer2.Close()
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
		t.Fatal("Couldn't create", roundType, "round:", err)
	}

	peer1.StartAnnouncement(round)
	time.Sleep(time.Second)

	var cosi *sign.CosiStruct
	switch roundType {
	case sign.RoundCosiType:
		cosi = round.(*sign.RoundCosi).Cosi
	case sign.RoundExceptionType:
		cosi = round.(*sign.RoundException).Cosi
	case conode.RoundStamperType:
		cosi = round.(*conode.RoundStamper).Cosi
	case conode.RoundStamperListenerType:
		cosi = round.(*conode.RoundStamperListener).Cosi
	}
	if cosi.R_hat == nil {
		t.Fatal("Didn't finish round - R_hat empty")
	}
	err = cosi.VerifyResponses()
	if err != nil {
		t.Fatal("Couldn't verify responses")
	}

	peer1.Close()
	peer2.Close()
}
