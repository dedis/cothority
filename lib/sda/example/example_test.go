package example_test

import (
	"github.com/dedis/cothority/lib/sda"
	"testing"
)

// Tests a 2-node system
func TestNode2(t *testing.T) {
	h1, priv1 := sda.NewHostKey("localhost:2000")
	h2, priv2 := sda.NewHostKey("localhost:2000")
	defer h1.Close()
	defer h2.Close()

	list := sda.NewEntityList([]*sda.EntityList{h1, h2})
	tree := list.
}

// Tests a 10-node system
