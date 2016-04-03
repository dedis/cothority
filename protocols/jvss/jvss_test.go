package jvss_test

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/jvss"
)

func TestJVSS(t *testing.T) {

	// Setup parameters
	var name string = "JVSS" // Protocol name
	var nodes uint32 = 10    // Number of nodes

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(int(nodes), false, true, true)
	defer local.CloseAll()

	dbg.TestOutput(true, 2)

	dbg.Lvl1("JVSS - starting")
	leader, err := local.CreateNewNodeName(name, tree)
	if err != nil {
		t.Fatal("Couldn't initialise protocol tree:", err)
	}
	jv := leader.ProtocolInstance().(*jvss.JVSS)
	leader.StartProtocol()

	dbg.Lvl1("JVSS - setup done")
	dbg.Lvl1("JVSS - requesting signature")
	msg := []byte("Hello World!")
	sig, _ := jv.Sign(msg)
	dbg.Lvl1("JVSS - signature received")
	err = jv.Verify(msg, sig)
	if err != nil {
		t.Fatal("Error signature verification failed", err)
	}
	dbg.Lvl1("JVSS - signature verification succeded")

}
