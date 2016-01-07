package sda_test

import (
	"crypto/sha256"
	"strconv"
	"testing"

	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/edwards/ed25519"
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
	pl := sda.PeerList{}
	app.ReadTomlConfig(&pl, "2_peers.toml", "testdata")
	if len(pl.Peers) != 2 {
		t.Fatalf("Expected two peers in PeerList. Instead got %d", len(pl.Peers))
	}
	// XXX do we want the ID to be regenerated, or do we want to read it from file, or both cases?
	// this test should define what behaviour is to expect ...
	if pl.Id == nil {
		t.Fatal("PeerList without ID is not allowed")
	}
	if len(pl.Id) != sha256.Size {
		t.Fatal("PeerList ID does not seem to be a sha256 hash.")
	}
	pl2 := sda.PeerList{}
	app.ReadTomlConfig(&pl2, "3_peers.toml", "testdata")
	if len(pl2.Peers) != 3 {
		t.Fatalf("Expected 3 peers in PeerList. Instead got %d", len(pl.Peers))
	}
	if pl.Id == nil {
		t.Fatal("PeerList without ID is not allowed")
	}
	if len(pl.Id) != sha256.Size {
		t.Fatal("PeerList ID does not seem to be a sha256 hash.")
	}
}

// Test initialisation of new random graph from a peer-list
func TestInitGraphFromPeerList(t *testing.T) {
	// NArra
	//adresses := genLocalhostPeerNames(3, 1010)
	//pl := sda.GenPeerList(tSuite, adresses)
	//sda.G
}

// Test initialisation of new graph from config-file using a peer-list
// XXX again this test might be obsolete/does more harm then it is useful:
// It forces every field to be exported/made public
// and we want to get away from config files (or not?)

// Test initialisation of new graph when one peer is represented more than
// once

// Test access to graph:
// - parent
// - children
// - public keys
// - corner-case: accessing parent/children with multiple instances of the same peer
// in the graph

// genLocalhostPeerNames will generate n localhost names with port indices starting from p
func genLocalhostPeerNames(n, p int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = "localhost" + strconv.Itoa(p+i)
	}
	return names
}
