package manage_test

import (
	"testing"

	"time"

	"github.com/dedis/cothority/sda"
)

// Tests a 2-node system
func TestCloseAll(t *testing.T) {
	local := sda.NewLocalTest()
	nbrNodes := 2
	_, _, tree := local.GenTree(nbrNodes, false, true, true)

	pi, err := local.CreateProtocol(tree, "CloseAll")
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	done := make(chan bool)
	go func() {
		pi.Start()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Didn't finish in 10 seconds")
	}
}
