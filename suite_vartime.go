// +build vartime

package cothority

import "github.com/dedis/kyber/suites"

// Suite is a convenience. It might go away when we decide the a better way to set the
// suite in repo cothority.
var Suite = suites.MustFind("P256")
