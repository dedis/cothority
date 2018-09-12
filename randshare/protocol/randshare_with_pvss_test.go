package randsharepvss

import (
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"testing"
	"time"
)

func TestRandShare(t *testing.T) {

	var name = "RandShare"
	var nodes = 13
	// 2/3 would prevent network splitting attacks
	var faulty = nodes / 3
	var purpose = "RandShare test run"

	local := onet.NewLocalTest()
	_, _, tree := local.GenTree(nodes, true)
	defer local.CloseAll()

	log.Lvlf1("RandShare starting")
	protocol, err := local.CreateProtocol(name, tree)
	if err != nil {
		t.Fatal("couldn't initialize", err)
	}
	rs := protocol.(*RandShare)
	startingTime := time.Now().Unix()
	err = rs.Setup(nodes, faulty, purpose, startingTime)
	if err != nil {
		t.Fatal("couldn't initialize", err)
	}

	err = rs.Start()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-rs.Done:
		random, transcript, err := rs.Random()
		if err != nil {
			t.Fatal(err)
		}
		if err = Verify(random, transcript); err != nil {
			t.Fatal(err)
		}
		log.Lvlf1("RandShare verified")
	case <-time.After(time.Second * time.Duration(nodes) * 2):
		t.Fatal("RandShare timeout")
	}
}
