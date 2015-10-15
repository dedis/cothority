package app

import (
	"github.com/dedis/cothority/lib/graphs"
)

type ConfigConode struct {
	// Coding-suite to run 	[nist256, nist512, ed25519]
	Suite string
	// Tree for knowing whom to connect
	Tree *graphs.Tree
}
