package example_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"testing"
)

// Tests a 2-node system
func TestNode2(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)

	h1, h2 := setupHosts(t, true)
	defer h1.Close()
	defer h2.Close()

	list := sda.NewEntityList([]*network.Entity{h1.Entity, h2.Entity})
	tree, _ := list.GenerateBinaryTree()
	h1.AddEntityList(list)
	h1.AddTree(tree)

	_, err := h1.StartNewProtocolName("Example", tree.Id)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
}

// Tests a 10-node system

func newHost(address string) *sda.Host {
	priv, pub := config.NewKeyPair(edwards.NewAES128SHA256Ed25519(false))
	id := network.NewEntity(pub, address)
	return sda.NewHost(id, priv)
}

// Creates two hosts on the local interfaces,
func setupHosts(t *testing.T, h2process bool) (*sda.Host, *sda.Host) {
	dbg.TestOutput(testing.Verbose(), 4)
	h1 := newHost("localhost:2000")
	// make the second peer as the server
	h2 := newHost("localhost:2001")
	h2.Listen()
	// make it process messages
	if h2process {
		go h2.ProcessMessages()
	}
	_, err := h1.Connect(h2.Entity)
	if err != nil {
		t.Fatal(err)
	}
	return h1, h2
}
