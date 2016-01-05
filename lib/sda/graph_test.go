package sda_test

import (
	"testing"

	"github.com/DeDiS/crypto/edwards/ed25519"
	"github.com/dedis/cothority/lib/sda"
)

var tSuite = ed25519.NewAES128SHA256Ed25519(false)

// Test initialisation of new peer-list
func TestInitPeerList(t *testing.T) {
	adresses := []string{"localhost:1010", "localhost:1012"}
	pl := sda.GenPeerList(tSuite, adresses)
	if len(pl.Peers) != 2 {
		t.Fatalf("Expected two peers in PeerList. Instead got %d", len(pl.Peers))
	}
	if pl.Id == nil {
		t.Fatalf("PeerList without id is not allowed")
	}
}

// Test initialisation of new peer-list from config-file

// Test initialisation of new random graph from a peer-list

// Test initialisation of new graph from config-file using a peer-list

// Test initialisation of new graph when one peer is represented more than
// once

// Test access to graph:
// - parent
// - children
// - public keys
// - corner-case: accessing parent/children with multiple instances of the same peer
// in the graph
