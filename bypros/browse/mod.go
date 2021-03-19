package browse

import (
	"context"

	"go.dedis.ch/cothority/v3/skipchain"
)

// Handler defines the type of function called on each block
type Handler func(block *skipchain.SkipBlock) error

// Service defines the primitive of a browsing service.
type Service interface {
	// GetBrowser returns a new browsing actor
	GetBrowser(handler Handler, id skipchain.SkipBlockID, target string) Actor
}

// Actor defines the primitives of a browsing actor. An actor is created with a
// browsing service.
type Actor interface {
	// Browse should call the service's handler on each block, starting from the
	// fromBlock until the end of the chain, or when the context is done.
	Browse(ctx context.Context, fromBlock skipchain.SkipBlockID) error
}
