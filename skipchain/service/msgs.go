package service

import (
	"github.com/dedis/cothority/skipchain/libsc"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	network.RegisterMessages(
		// - Internal calls
		// Propagation
		&PropagateSkipBlocks{},
		// Request forward-signature
		&ForwardSignature{},
		// Request updated block
		&GetBlock{},
		// Reply with updated block
		&GetBlockReply{})
}

// Internal calls

// PropagateSkipBlocks sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlocks struct {
	SkipBlocks []*libsc.SkipBlock
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
	Previous libsc.SkipBlockID
	// Newest is the newest skipblock, signed by previous
	Newest *libsc.SkipBlock
	// ForwardLink is the signature from Previous to Newest
	ForwardLink *libsc.BlockLink
}

// GetBlock asks for an updated block, in case for a conode that is not
// in the roster-list of that block.
type GetBlock struct {
	ID libsc.SkipBlockID
}

// PropagateSkipBlock sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlock struct {
	SkipBlock *libsc.SkipBlock
}

// GetBlockReply returns the requested block.
type GetBlockReply struct {
	SkipBlock *libsc.SkipBlock
}
