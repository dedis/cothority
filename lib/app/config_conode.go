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

	////// Only used during process and never written to file /////
	// Private / public keys
	Secret abstract.Secret
	Public abstract.Point
}
