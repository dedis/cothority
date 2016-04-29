package skipchain

import (
	"bytes"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

type SkipBlockID crypto.HashID

func (sbid SkipBlockID) IsNull() bool {
	return len(sbid) == 0
}

// SkipBlockFix represents the fixed part of a SkipBlock that will be hashed
// and signed.
type SkipBlockFix struct {
	Index uint32
	// Height of that SkipBlock
	Height uint32
	// For deterministic SkipChains at what height to stop:
	// - if negative: we will use random distribution to calculate the
	// height of each new block
	// - else: the max height determines the height of the next block
	MaximumHeight int
	// BackLink is a slice of hashes to previous SkipBlocks
	BackLinkIds []SkipBlockID
	// VerifierId is a SkipBlock-protocol verifying new SkipBlocks
	VerifierId VerifierID
	// SkipBlockParent points to the SkipBlock of the responsible Roster -
	// is nil if this is the Root-roster
	ParentBlockId SkipBlockID
	// Aggregate is the aggregate key of our responsible roster
	Aggregate abstract.Point
	// Data is any data to be stored in that SkipBlock
	Data []byte
	// EntityList holds the roster-definition of that SkipBlock
	EntityList *sda.EntityList
}

// addSliceToHash hashes the whole SkipBlockFix plus a slice of bytes.
// This is used
func (sbf *SkipBlockFix) calculateHash() SkipBlockID {
	b, err := network.MarshalRegisteredType(sbf)
	if err != nil {
		dbg.Panic("Couldn't marshal SkipBlockFix:", err)
	}
	h, err := crypto.HashBytes(network.Suite.Hash(), b)
	if err != nil {
		dbg.Panic("Couldn't hash SkipBlockFix:", err)
	}
	return h
}

// SkipBlock represents a SkipBlock of any type - the fields that won't
// be hashed (yet).
type SkipBlock struct {
	*SkipBlockFix
	// This is our block Hash and Signature
	*BlockLink
	// ForwardLink will be calculated once future SkipBlocks are
	// available
	ForwardLink []*BlockLink
	// SkipLists that depend on us, given as the first SkipBlock - can
	// be a Data or a Roster SkipBlock
	ChildSL *BlockLink
}

// NewSkipBlockCommon pre-initialises the block so it can be sent over
// the network
func NewSkipBlock() *SkipBlock {
	return &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			Data: make([]byte, 0),
		},
		BlockLink: NewBlockLink(),
	}
}

// VerifySignature returns whether all signatures are correctly signed
// by the aggregate public key of the roster. It needs the aggregate key.
func (sbc *SkipBlock) VerifySignatures() error {
	if err := sbc.BlockLink.VerifySignature(sbc.Aggregate); err != nil {
		return err
	}
	for _, fl := range sbc.ForwardLink {
		if err := fl.VerifySignature(sbc.Aggregate); err != nil {
			return err
		}
	}
	if sbc.ChildSL != nil && sbc.ChildSL.Hash == nil {
		return sbc.ChildSL.VerifySignature(sbc.Aggregate)
	}
	return nil
}

// Equal returns bool if both hashes are equal
func (sbc *SkipBlock) Equal(sb *SkipBlock) bool {
	return bytes.Equal(sbc.Hash, sb.Hash)
}

func (sbc *SkipBlock) updateHash() SkipBlockID {
	sbc.Hash = sbc.calculateHash()
	return sbc.Hash
}

// BlockLink has the hash and a signature of a block
type BlockLink struct {
	Hash SkipBlockID
	*cosi.Signature
}

// NewBlockLink pre-initialises the signature so it can be sent
// over the network.
func NewBlockLink() *BlockLink {
	return &BlockLink{
		Signature: cosi.NewSignature(network.Suite),
	}
}

// VerifySignature returns whether the BlockLink has been signed
// correctly using the aggregate key given.
func (bl *BlockLink) VerifySignature(aggregate abstract.Point) error {
	// TODO: enable real verification once we have signatures
	return nil
	//return cosi.VerifySignature(network.Suite, bl.Hash, aggregate,
	//	bl.Challenge, bl.Response)
}
