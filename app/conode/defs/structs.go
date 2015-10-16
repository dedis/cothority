package defs
import (
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/crypto/abstract"
)

type StampSend struct {
	// The hash-message we want to be signed
	Message hashid.HashId
}

type StampRcv struct {
	// The aggregated public key
	K0  abstract.Point
	// The hash over T0 of the root-node
	GID hashid.HashId
	// Challenge of Shamir
	C   abstract.Secret
	// Aggregated response
	R0  abstract.Secret

	// Unknown territory
	// Inclusion-proof of the Inclusion-Merkle-tree
	IP  []hashid.HashId
}