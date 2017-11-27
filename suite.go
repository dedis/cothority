package cothority

import (
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/group"
)

type suite interface {
	kyber.Group
	kyber.HashFactory
}

// This is a temporary hack until we decide the right way to
// set the suite in repo cothority.
var Suite suite

func init() {
	Suite = group.MustSuite("Ed25519").(suite)
}
