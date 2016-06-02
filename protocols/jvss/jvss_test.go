package jvss_test

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/jvss"
)

func TestJVSS(t *testing.T) {

	// Setup parameters
	var name string = "JVSS"      // Protocol name
	var nodes uint32 = 5          // Number of nodes
	var rounds int = 3            // Number of rounds
	msg := []byte("Hello World!") // Message to-be-signed

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(int(nodes), false, true, true)
	defer local.CloseAll()

	dbg.TestOutput(testing.Verbose(), 1)

	dbg.Lvl1("JVSS - starting")
	leader, err := local.CreateProtocol(name, tree)
	if err != nil {
		t.Fatal("Couldn't initialise protocol tree:", err)
	}
	jv := leader.(*jvss.JVSS)
	leader.Start()
	dbg.Lvl1("JVSS - setup done")

	for i := 0; i < rounds; i++ {
		dbg.Lvl1("JVSS - starting round", i)
		dbg.Lvl1("JVSS - requesting signature")
		sig, _ := jv.Sign(msg)
		dbg.Lvl1("JVSS - signature received")
		err = jv.Verify(msg, sig)
		if err != nil {
			t.Fatal("Error signature verification failed", err)
		}
		dbg.Lvl1("JVSS - signature verification succeded")
	}

}
