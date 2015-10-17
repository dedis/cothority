package app

import (
	"github.com/dedis/cothority/lib/graphs"
	"github.com/dedis/crypto/abstract"
)

type ConfigConode struct {
	// Coding-suite to run 	[nist256, nist512, ed25519]
	Suite string
	// Tree for knowing whom to connect
	Tree *graphs.Tree
	// hosts
	Hosts []string

	// Aggregated long term public keys of all the peers in the tree
	AggPubKey string
	////// Only used during process and never written to file /////
	// Private / public keys of your host
	Secret abstract.Secret
	Public abstract.Point
}
