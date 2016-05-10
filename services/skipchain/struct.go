package skipchain

import (
	"bytes"

	"fmt"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

// SkipBlockID represents the Hash of the SkipBlock
type SkipBlockID crypto.HashID

// IsNull returns true if the ID is undefined
func (sbid SkipBlockID) IsNull() bool {
	return len(sbid) == 0
}

func (sbid SkipBlockID) String() string {
	if sbid.IsNull() {
		return "Nil"
	}
	return fmt.Sprintf("%x", []byte(sbid[0:8]))
}

// SkipBlockFix represents the fixed part of a SkipBlock that will be hashed
// and signed.
type SkipBlockFix struct {
	Index int
	// Height of that SkipBlock
	Height int
	// The max height determines the height of the next block
	MaximumHeight int
	// For deterministic SkipChains, chose a value >= 1 - higher
	// bases mean more 'height = 1' SkipBlocks
	// For random SkipChains, chose a value of 0
	BaseHeight int
	// BackLink is a slice of hashes to previous SkipBlocks
	BackLinkIds []SkipBlockID
	// VerifierID is a SkipBlock-protocol verifying new SkipBlocks
	VerifierID VerifierID
	// SkipBlockParent points to the SkipBlock of the responsible Roster -
	// is nil if this is the Root-roster
	ParentBlockID SkipBlockID
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

// NewSkipBlock pre-initialises the block so it can be sent over
// the network
func NewSkipBlock() *SkipBlock {
	return &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			Data: make([]byte, 0),
		},
		BlockLink: NewBlockLink(),
	}
}

// VerifySignatures returns whether all signatures are correctly signed
// by the aggregate public key of the roster. It needs the aggregate key.
func (sb *SkipBlock) VerifySignatures() error {
	if err := sb.BlockLink.VerifySignature(sb.Aggregate); err != nil {
		return err
	}
	for _, fl := range sb.ForwardLink {
		if err := fl.VerifySignature(sb.Aggregate); err != nil {
			return err
		}
	}
	if sb.ChildSL != nil && sb.ChildSL.Hash == nil {
		return sb.ChildSL.VerifySignature(sb.Aggregate)
	}
	return nil
}

// Equal returns bool if both hashes are equal
func (sb *SkipBlock) Equal(other *SkipBlock) bool {
	return bytes.Equal(sb.Hash, other.Hash)
}

// Copy makes a deep copy of the SkipBlock
func (sb *SkipBlock) Copy() *SkipBlock {
	b := *sb
	sbf := *b.SkipBlockFix
	b.SkipBlockFix = &sbf
	b.BlockLink = b.BlockLink.Copy()
	b.ForwardLink = make([]*BlockLink, len(sb.ForwardLink))
	for i, fl := range sb.ForwardLink {
		b.ForwardLink[i] = fl.Copy()
	}
	if b.ChildSL != nil {
		b.ChildSL = sb.ChildSL.Copy()
	}
	return &b
}

func (sb *SkipBlock) String() string {
	return sb.Hash.String()
}

func (sb *SkipBlock) updateHash() SkipBlockID {
	sb.Hash = sb.calculateHash()
	return sb.Hash
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

// Copy makes a deep copy of a blocklink
func (bl *BlockLink) Copy() *BlockLink {
	sig := &cosi.Signature{
		Challenge: network.Suite.Secret().Set(bl.Challenge),
		Response:  network.Suite.Secret().Set(bl.Response),
	}
	return &BlockLink{
		Hash:      bl.Hash,
		Signature: sig,
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
