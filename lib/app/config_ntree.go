package app

import (
	"github.com/dedis/cothority/lib/graphs"
)

type NTreeConfig struct {
	Hpn int

	Bf int

	Suite string

	Rounds int

	Debug int

	Hosts []string

	Tree *graphs.Tree

	Name string

	Root bool

	SkipChecks bool
}
