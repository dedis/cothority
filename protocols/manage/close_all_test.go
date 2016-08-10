package manage_test

import (
	"testing"

	"github.com/dedis/cothority/sda"
)

// Tests a 2-node system
func TestCloseAll(t *testing.T) {
	local := sda.NewLocalTest()
	nbrNodes := 2
	_, _, tree := local.GenTree(nbrNodes, false, true, true)

	pi, err := local.CreateProtocol("CloseAll", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	pi.Start()
}
