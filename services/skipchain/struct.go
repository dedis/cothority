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

type SkipBlock interface {
	// updateHash updates the hash, stores it in the SkipBlock and return the
	// resulting hash to the caller
	updateHash() SkipBlockID
	// VerifySignature checks if the main signature and all forward-links
	// are correctly signed and returns an error if not.
	VerifySignatures() error
	// GetCommon returns the part of the main information about the
	// SkipBlock which is the SkipBlockCommon structure.
	GetCommon() *SkipBlockCommon
	// Hash returns the hash of the SkipBlock which is its ID
	GetHash() SkipBlockID
	// Equal tests if both hashes are equal
	Equal(SkipBlock) bool
}

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
	BackLink []SkipBlockID
	// VerifierId is a SkipBlock-protocol verifying new SkipBlocks
	VerifierId VerifierID
	// SkipBlockParent points to the SkipBlock of the responsible Roster -
	// is nil if this is the Root-roster
	ParentBlock SkipBlockID
}

// addSliceToHash hashes the whole SkipBlockFix plus a slice of bytes.
// This is used
func (sbf *SkipBlockFix) addSliceToHash(slice []byte) SkipBlockID {
	b, err := network.MarshalRegisteredType(sbf)
	if err != nil {
		dbg.Panic("Couldn't marshal SkipBlockFix:", err)
	}
	h, err := crypto.HashBytes(network.Suite.Hash(), append(b, slice...))
	if err != nil {
		dbg.Panic("Couldn't hash SkipBlockFix:", err)
	}
	return h
}

// SkipBlockCommon represents a SkipBlock of any type - the fields that won't
// be hashed (yet).
type SkipBlockCommon struct {
	*SkipBlockFix
	// This is our block Hash and Signature
	*BlockLink
	// Aggregate is the aggregate key of our responsible roster
	Aggregate abstract.Point
	// ForwardLink will be calculated once future SkipBlocks are
	// available
	ForwardLink []*BlockLink
}

// NewSkipBlockCommon pre-initialises the block so it can be sent over
// the network
func NewSkipBlockCommon() *SkipBlockCommon {
	return &SkipBlockCommon{
		SkipBlockFix: &SkipBlockFix{},
		BlockLink:    NewBlockLink(),
	}
}

// VerifySignature returns whether all signatures are correctly signed
// by the aggregate public key of the roster. It needs the aggregate key.
func (sbc *SkipBlockCommon) VerifySignatures() error {
	if err := sbc.BlockLink.VerifySignature(sbc.Aggregate); err != nil {
		return err
	}
	for _, fl := range sbc.ForwardLink {
		if err := fl.VerifySignature(sbc.Aggregate); err != nil {
			return err
		}
	}
	return nil
}

// GetCommon returns the common skipblock part from the interface.
func (sbc *SkipBlockCommon) GetCommon() *SkipBlockCommon {
	return sbc
}

// Hash returns the hash of the SkipBlock
func (sbc *SkipBlockCommon) GetHash() SkipBlockID {
	return sbc.Hash
}

// Equal returns bool if both hashes are equal
func (sbc *SkipBlockCommon) Equal(sb SkipBlock) bool {
	return bytes.Equal(sbc.Hash, sb.GetHash())
}

// SkipBlockData is a SkipBlock that can hold some data.
type SkipBlockData struct {
	*SkipBlockCommon
	// Data is any data to be stored in that SkipBlock
	Data []byte
}

// NewSkipBlockData initialises a SkipBlockData so that it can be sent
// over the network
func NewSkipBlockData() *SkipBlockData {
	return &SkipBlockData{
		SkipBlockCommon: NewSkipBlockCommon(),
	}
}

// updateHash is used to store the hash of the SkipBlockFix and the
// data.
func (sbd SkipBlockData) updateHash() SkipBlockID {
	sbd.Hash = sbd.addSliceToHash(sbd.Data)
	return sbd.Hash
}

// SkipBlockRoster is a SkipBlock tracking different Rosters
type SkipBlockRoster struct {
	*SkipBlockCommon
	// EntityList holds the roster-definition of that SkipBlock
	EntityList *sda.EntityList
	// SkipLists that depend on us, given as the first SkipBlock - can
	// be a Data or a Roster SkipBlock
	ChildSL *BlockLink
}

// NewSkipBlockRoster initialises a SkipBlockRoster
func NewSkipBlockRoster(el *sda.EntityList) *SkipBlockRoster {
	return &SkipBlockRoster{
		SkipBlockCommon: NewSkipBlockCommon(),
		EntityList:      el,
		ChildSL: &BlockLink{
			Signature: cosi.NewSignature(network.Suite),
		},
	}
}

// updateHash is used to store the hash of the SkipBlockFix and the
// EntityList
func (sbr SkipBlockRoster) updateHash() SkipBlockID {
	sbr.Hash = sbr.addSliceToHash(sbr.EntityList.Id[:])
	return sbr.Hash
}

// VerifySignatures checks the main hash and all available ForwardLinks. If this
// structure holds a ChildSkipList, its signature will also be verified.
func (sbr SkipBlockRoster) VerifySignatures() error {
	err := sbr.SkipBlockCommon.VerifySignatures()
	if err != nil || (sbr.ChildSL != nil && sbr.ChildSL.Hash == nil) {
		return err
	}
	return sbr.ChildSL.VerifySignature(sbr.Aggregate)
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
