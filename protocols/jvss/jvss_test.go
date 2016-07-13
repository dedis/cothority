package jvss_test

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/protocols/jvss"
	"github.com/dedis/cothority/sda"
)

func TestJVSS(t *testing.T) {
	// Setup parameters
	var name string = "JVSS"      // Protocol name
	var nodes uint32 = 17         // Number of nodes
	var rounds int = 16           // Number of rounds
	msg := []byte("Hello World!") // Message to-be-signed

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(int(nodes), false, true, true)
	defer local.CloseAll()

	log.TestOutput(testing.Verbose(), 2)

	log.Lvl1("JVSS - starting")
	leader, err := local.CreateProtocol(tree, name)
	if err != nil {
		t.Fatal("Couldn't initialise protocol tree:", err)
	}
	jv := leader.(*jvss.JVSS)
	leader.Start()
	log.Lvl1("JVSS - setup done")

	for i := 0; i < rounds; i++ {
		log.Lvl1("JVSS - starting round", i)
		log.Lvl1("JVSS - requesting signature")
		sig, err := jv.Sign(msg)
		if err != nil {
			t.Fatal("Error signature failed", err)
		}
		log.Lvl1("JVSS - signature received")
		err = jv.Verify(msg, sig)
		if err != nil {
			t.Fatal("Error signature verification failed", err)
		}
		log.Lvl1("JVSS - signature verification succeded")
	}

}
