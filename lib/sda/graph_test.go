package sda_test

import (
	"crypto/sha256"
	"testing"

	"github.com/DeDiS/crypto/edwards/ed25519"
	"github.com/dedis/cothority/lib/sda"
)

var tSuite = ed25519.NewAES128SHA256Ed25519(false)

// Test initialisation of new peer-list
func TestInitPeerList(t *testing.T) {
	adresses := []string{"localhost:1010", "localhost:1012"}
	pl := sda.GenPeerList(tSuite, adresses)
	if pl == nil {
		t.Fatal("Could not initialize PeerList")
	}
	if len(pl.Peers) != 2 {
		t.Fatalf("Expected two peers in PeerList. Instead got %d", len(pl.Peers))
	}
	if pl.Id == nil {
		t.Fatal("PeerList without ID is not allowed")
	}
	if len(pl.Id) != sha256.Size {
		t.Fatal("PeerList ID does not seem to be a sha256 hash.")
	}
}

// Test initialisation of new peer-list from config-file
func TestInitPeerListFromConfigFile(t *testing.T) {
	// XXX ask Linus why this test is necessary/useful
	// (config files should be obsolete)
	// pl := sda.PeerList{}
	// app.ReadTomlConfig(&pl, "peerlist.toml", "testdata")
	// if len(pl.Peers) != 2 {
	// 	t.Fatalf("Expected two peers in PeerList. Instead got %d", len(pl.Peers))
	// }
	// if pl.Id == nil { // XXX do we want the ID to be regenerated, or do we want to read it from file, or both cases?
	// 	t.Fatal("PeerList without ID is not allowed")
	// }
	// if len(pl.Id) != sha256.Size {
	// 	t.Fatal("PeerList ID does not seem to be a sha256 hash.")
	// }
}

// Test initialisation of new random graph from a peer-list
func TestInitGraphFromPeerList(t *testing.T) {
}

// Test initialisation of new graph from config-file using a peer-list

// Test initialisation of new graph when one peer is represented more than
// once

// Test access to graph:
// - parent
// - children
// - public keys
// - corner-case: accessing parent/children with multiple instances of the same peer
// in the graph
