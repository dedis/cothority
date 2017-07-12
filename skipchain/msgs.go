package skipchain

import "gopkg.in/dedis/onet.v1/network"

func init() {
	for _, m := range []interface{}{
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
	} {
		network.RegisterMessage(m)
	}
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
