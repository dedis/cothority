package app

import (
	"github.com/dedis/crypto/abstract"
)

type ConfigSkeleton struct {
	*ConfigConode
	// ppm is the replication factor of hosts per node: how many hosts do we want per node
	Ppm int
	// bf is the branching factor of the tree that we build
	Bf int

	// How many rounds
	Rounds int
	// Debug-level
	Debug int

	////// Only used during process and never written to file /////
	// Private / public keys of your host
	Secret abstract.Secret
	Public abstract.Point
}
