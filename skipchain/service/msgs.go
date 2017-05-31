package service

import (
	"github.com/dedis/cothority/skipchain"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	for _, m := range []interface{}{
		// - Internal calls
		// Propagation
		&PropagateSkipBlocks{},
		// Request forward-signature
		&ForwardSignature{},
		// Request updated block
		&GetBlock{},
		// Reply with updated block
		&GetBlockReply{},
	} {
		network.RegisterMessage(m)
	}
}

// Internal calls

// PropagateSkipBlocks sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlocks struct {
	SkipBlocks []*skipchain.SkipBlock
}

// ForwardSignature is called once a new skipblock has been accepted by
// signing the forward-link, and then the older skipblocks need to
// update their forward-links. Each cothority needs to get the necessary
// blocks and propagate the skipblocks itself.
type ForwardSignature struct {
	// TargetHeight is the index in the backlink-slice of the skipblock
	// to update
	TargetHeight int
	// Previous is the second-newest skipblock
	Previous skipchain.SkipBlockID
	// Newest is the newest skipblock, signed by previous
	Newest *skipchain.SkipBlock
	// ForwardLink is the signature from Previous to Newest
	ForwardLink *skipchain.BlockLink
}

// GetBlock asks for an updated block, in case for a conode that is not
// in the roster-list of that block.
type GetBlock struct {
	ID skipchain.SkipBlockID
}

// PropagateSkipBlock sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlock struct {
	SkipBlock *skipchain.SkipBlock
}

// GetBlockReply returns the requested block.
type GetBlockReply struct {
	SkipBlock *skipchain.SkipBlock
}
