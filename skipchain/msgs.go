package skipchain

import "gopkg.in/dedis/onet.v1/network"

func init() {
	network.RegisterMessages(
		// - API calls
		// Store new skipblock
		&StoreSkipBlock{},
		&StoreSkipBlockReply{},
		// Requests for data
		&GetBlocks{},
		&GetBlocksReply{},
		// Fetch all skipchains
		&GetAllSkipchains{},
		&GetAllSkipchainsReply{},
		// Get only one block
		&GetBlockByIndex{},
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

// This file holds all messages that can be sent to the SkipChain,
// both from the outside and between instances of this service

// External calls

// StoreSkipBlock - Requests a new skipblock to be stored on the skipchain.
// For more details, see skipchain.Client::StoreSkipBlock
type StoreSkipBlock struct {
	NewBlock *SkipBlock
}

// StoreSkipBlockReply - returns the signed SkipBlock with updated backlinks
type StoreSkipBlockReply struct {
	Previous *SkipBlock
	Latest   *SkipBlock
}

// GetBlocks - requests blocks from the skipchain. Different return-modes
// are possible: update, all, shortest. For more detail, see
// skipchain.Client::GetBlocks.
type GetBlocks struct {
	Start     SkipBlockID
	End       SkipBlockID
	MaxHeight int
}

// GetBlocksReply - returns the request from the GetBlocks.
type GetBlocksReply struct {
	Reply []*SkipBlock
}

// GetAllSkipchains - returns all known last blocks of skipchains.
type GetAllSkipchains struct {
}

// GetAllSkipchainsReply - returns all known last blocks of skipchains.
type GetAllSkipchainsReply struct {
	SkipChains []*SkipBlock
}

// GetBlockByIndex asks for a single block.
type GetBlockByIndex struct {
	Genesis SkipBlockID
	Index   int
}

// Internal calls

// PropagateSkipBlocks sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlocks struct {
	SkipBlocks []*SkipBlock
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
	Previous SkipBlockID
	// Newest is the newest skipblock, signed by previous
	Newest *SkipBlock
	// ForwardLink is the signature from Previous to Newest
	ForwardLink *BlockLink
}

// GetBlock asks for an updated block, in case for a conode that is not
// in the roster-list of that block.
type GetBlock struct {
	ID SkipBlockID
}

// PropagateSkipBlock sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlock struct {
	SkipBlock *SkipBlock
}

// GetBlockReply returns the requested block.
type GetBlockReply struct {
	SkipBlock *SkipBlock
}
