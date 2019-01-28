// +build !vartime

package cothority

import "go.dedis.ch/kyber/v3/suites"

// Suite is a convenience. It might go away when we decide the a better way to set the
// suite in repo cothority.
var Suite = suites.MustFind("Ed25519")
