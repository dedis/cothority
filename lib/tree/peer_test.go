package tree_test

import (
	"github.com/dedis/cothority/lib/tree"
	"github.com/dedis/crypto/edwards"
	"testing"
)

func TestNewPeerListLocal(t *testing.T) {
	s := edwards.NewAES128SHA256Ed25519(false)
	pl := tree.NewPeerListLocalhost(s, 11, 2000)
	if len(pl.Peers) != 11 {
		t.Fatal("Did not get 11 peers")
	}
}
