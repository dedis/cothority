package randhound_test

import (
	"log"
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/randhound"
)

func TestRandHound(t *testing.T) {

	// Setup parameters
	var name string = "RandHound"       // Protocol name
	var np int = 10                     // Number of peers (including leader)
	var T, R, N int = 3, 3, 5           // VSS parameters (T <= R <= N)
	var p string = "RandHound test run" // Purpose

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(np, false, true, true)
	defer local.CloseAll()

	dbg.TestOutput(testing.Verbose(), 1)

	// Start RandHound
	log.Printf("RandHound - starting")
	node, err := local.CreateNewNodeName(name, tree)
	if err != nil {
		t.Fatal("Couldn't start RandHound protocol:", err)
	}
	rh := node.ProtocolInstance().(*randhound.RandHound)
	rh.T = T
	rh.R = R
	rh.N = N
	rh.Purpose = p

	bytes := make([]byte, 32)
	select {
	case _ = <-rh.Done:
		log.Printf("RandHound - done")
		bytes = <-rh.Result
	case <-time.After(time.Second * 60):
		t.Fatal("RandHound – time out")
	}
	log.Printf("RandHound - random bytes: %v\n", bytes)
}
