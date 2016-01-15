package app

import (
	"github.com/dedis/cothority/lib/tree"
)

type NTreeConfig struct {
	Ppm int

	Bf int

	Suite string

	Rounds int

	Debug int

	Hosts []string

	Tree *tree.ConfigTree

	Name string

	Root bool

	SkipChecks bool
}
