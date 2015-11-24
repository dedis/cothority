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

	if len(peer1.Rounds) != 0{
		t.Fatal("There should be 0 rounds to start with")
	}

	round, err := sign.NewRoundFromType("cosistamper", peer1.Node)
	if err != nil{
		t.Fatal("Couldn't create cosi-round")
	}

	peer1.StartAnnouncement(round)
	if len(peer1.Rounds) != 1{
		t.Fatal("Created one round - should be there")
	}

	time.Sleep(time.Second)

	if len(peer1.Rounds) != 0{
		t.Fatal("Doing one round shouldn't take more than 1 second")
	}

	peer1.Close()
	peer2.Close()
}

// Tests the cosi-round
func TestRoundCosi(t *testing.T){
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()

	round, err := sign.NewRoundFromType("cosi", peer1.Node)
	if err != nil{
		t.Fatal("Couldn't create cosi-round")
	}

	peer1.StartAnnouncement(round)

	peer1.Close()
	peer2.Close()
}