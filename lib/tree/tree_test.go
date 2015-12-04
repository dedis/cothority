package tree_test

import (
	"bytes"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/tree"
	"github.com/dedis/crypto/edwards"
	"testing"
)

func TestNewNaryTree(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	s := edwards.NewAES128SHA256Ed25519(false)
	pl := tree.NewPeerList(s, []string{"localhost"}, 11, 2000)
	tr := pl.NewNaryTree(3)
	// Count all elements
	if tr.Count() != 11 {
		t.Fatal("Not 11 elements")
	}

	// Check same hash for same tree
	tr2 := pl.NewNaryTree(3)
	if bytes.Compare(tr.Hash, tr2.Hash) != 0 {
		t.Fatal("Hashes of same tree are different")
	}

	// Check different hash for different trees
	tr3 := pl.NewNaryTree(4)
	if bytes.Compare(tr.Hash, tr3.Hash) == 0 {
		t.Fatal("Hashes of different trees should be different")
	}

	// Count for wider tree
	if tr3.Count() != 11 {
		t.Fatal("Not 11 elements for bf=4 tree")
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
