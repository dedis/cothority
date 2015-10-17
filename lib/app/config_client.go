package app

import (
	"github.com/dedis/crypto/abstract"
)

// Struct used by the client containing infromations to verify signatures
type ConfigClient struct {
	// Agreggated public keys !
	K0 abstract.Point
}
