package skipchain

import (
	"github.com/dedis/onet"
	"github.com/dedis/cothority/byzcoinx"
)

// PROTOSTART


// SkipBlock represents a SkipBlock of any type - the fields that won't
// be hashed (yet).
type SkipBlock struct {
	*SkipBlockFix
	// Hash is our Block-hash of the SkipBlockFix part.
	Hash SkipBlockID

	// ForwardLink will be calculated once future SkipBlocks are
	// available
	ForwardLink []*ForwardLink
	// SkipLists that depend on us, given as the first SkipBlock - can
	// be a Data or a Roster SkipBlock
	ChildSL []SkipBlockID

	// Payload is additional data that needs to be hashed by the application
	// itself into SkipBlockFix.Data. A normal usecase is to set
	// SkipBlockFix.Data to the sha256 of this payload. Then the proofs
	// using the skipblocks can return simply the SkipBlockFix, as long as they
	// don't need the payload.
	Payload []byte `protobuf:"opt"`
}

// SkipBlockFix represents the fixed part of a SkipBlock that will be hashed
// and signed.
type SkipBlockFix struct {
	// Index of the block in the chain. Index == 0 -> genesis-block.
	Index int
	// Height of that SkipBlock, starts at 1.
	Height int
	// The max height determines the height of the next block
	MaximumHeight int
	// For deterministic SkipChains, chose a value >= 1 - higher
	// bases mean more 'height = 1' SkipBlocks
	// For random SkipChains, chose a value of 0
	BaseHeight int
	// BackLink is a slice of hashes to previous SkipBlocks
	BackLinkIDs []SkipBlockID
	// VerifierID is a SkipBlock-protocol verifying new SkipBlocks
	VerifierIDs []VerifierID
	// SkipBlockParent points to the SkipBlock of the responsible Roster -
	// is nil if this is the Root-roster
	ParentBlockID SkipBlockID
	// GenesisID is the ID of the genesis-block. For the genesis-block, this
	// is null. The SkipBlockID() method returns the correct ID both for
	// the genesis block and for later blocks.
	GenesisID SkipBlockID
	// Data is any data to be stored in that SkipBlock
	Data []byte
	// Roster holds the roster-definition of that SkipBlock
	Roster *onet.Roster
}

// ForwardLink can be used to jump from old blocks to newer
// blocks. Depending on the BaseHeight and MaximumHeight, older
// rosters are asked to sign direct links to new blocks.
type ForwardLink struct {
	// From - where this forward link comes from
	From SkipBlockID
	// To - where this forward link points to
	To SkipBlockID
	// NewRoster is only set to non-nil if the From block has a
	// different roster from the To-block.
	NewRoster *onet.Roster
	// Signature is calculated on the
	// sha256(From.Hash()|To.Hash()|NewRoster)
	// In the case that NewRoster is nil, the signature is
	// calculated on the sha256(From.Hash()|To.Hash())
	Signature byzcoinx.FinalSignature
}

// GetUpdateChain - the client sends the hash of the last known
// Skipblock and will get back a list of all necessary SkipBlocks
// to get to the latest.
type GetUpdateChain struct {
	LatestID SkipBlockID
}

// GetUpdateChainReply - returns the shortest chain to the current SkipBlock,
// starting from the SkipBlock the client sent
type GetUpdateChainReply struct {
	Update []*SkipBlock
}


// GetAllSkipchains - returns all known last blocks of skipchains.
type GetAllSkipchains struct {
}

// GetAllSkipchainsReply - returns all known last blocks of skipchains.
type GetAllSkipchainsReply struct {
	SkipChains []*SkipBlock
}


// PropagateSkipBlocks sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlocks struct {
	SkipBlocks []*SkipBlock
}


// Internal calls

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
}

// GetSingleBlock asks for a single block.
type GetSingleBlock struct {
	ID SkipBlockID
}

// GetSingleBlockByIndex asks for a single block at a certain index. If Index == -1,
// the last block on the skipchain is returned.
type GetSingleBlockByIndex struct {
	Genesis SkipBlockID
	Index   int
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

// External calls

// StoreSkipBlock - Requests a new skipblock to be appended to the given
// SkipBlock. If the given TargetSkipChainID is an empty slice, then a genesis
// block is created.  Otherwise, the new block is added to the skipchain
// specified by TargetSkipChainID.
type StoreSkipBlock struct {
	TargetSkipChainID SkipBlockID
	NewBlock          *SkipBlock
	Signature         *[]byte
}

// StoreSkipBlockReply - returns the signed SkipBlock with updated backlinks
type StoreSkipBlockReply struct {
	Previous *SkipBlock
	Latest   *SkipBlock
}
