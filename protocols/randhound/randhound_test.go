package randhound_test

import (
	"log"
	"testing"
	"time"

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
	_, _, tree := local.GenTree(np, false, true)
	defer local.CloseAll()

	// Register RandHound
	fn := func(node *sda.Node) (sda.ProtocolInstance, error) {
		return randhound.NewRandHound(node, T, R, N, p)
	}
	sda.ProtocolRegisterName(name, fn)

	// Start RandHound
	log.Printf("RandHound - starting")
	node, err := local.StartNewNodeName(name, tree)
	if err != nil {
		t.Fatal("Couldn't start RandHound protocol:", err)
	}
	rh := node.ProtocolInstance().(*randhound.RandHound)

	bytes := make([]byte, 32)
	select {
	case _ = <-rh.Done:
		log.Printf("RandHound - done")
		bytes = <-rh.Result
	case <-time.After(time.Second * 10):
		t.Fatal("RandHound – time out")
	}
	log.Printf("RandHound - random bytes: %v\n", bytes)
}
