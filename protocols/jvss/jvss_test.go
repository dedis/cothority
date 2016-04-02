package jvss_test

import (
	"log"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/jvss"
)

func TestJVSS(t *testing.T) {

	// Setup parameters
	var name string = "JVSS" // Protocol name
	var nodes uint32 = 5     // Number of nodes

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(int(nodes), false, true, true)
	defer local.CloseAll()

	dbg.TestOutput(true, 1)

	log.Printf("JVSS - starting")
	leader, err := local.CreateNewNodeName(name, tree)
	if err != nil {
		t.Fatal("Couldn't initialise protocol tree:", err)
	}
	jv := leader.ProtocolInstance().(*jvss.JVSS)
	leader.StartProtocol()

	select {
	case <-jv.Done:
		log.Printf("JVSS - setup done")
	}

	msg := []byte("Hello World\n")
	sig, _ := jv.Sign(msg)

	err = jv.Verify(msg, sig)
	if err != nil {
		t.Fatal("Error signature verification failed", err)
	}
	log.Printf("JVSS - signature verification succeded")

}
