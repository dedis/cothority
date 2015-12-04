package tree_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/tree"
	"github.com/dedis/crypto/edwards"
	"testing"
)

func TestNewNaryTree(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	s := edwards.NewAES128SHA256Ed25519(false)
	pl := tree.NewPeerListLocalhost(s, 11, 2000)
	tr := tree.NewNaryTree(pl.Peers, 3)
	// Count all elements
	if tr.Count() != 11 {
		t.Fatal("Not 11 elements")
	}
}

func TestSlices(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	var a []int
	a = make([]int, 10)
	for i := range a {
		a[i] = i
	}
	dbg.Lvlf3("a[0..3] is %+v", a[1:3])
	dbg.Lvlf3("a[0..1] is %+v", a[0:1])
	dbg.Lvlf3("a[0..0] is %+v", a[0:0])
}
