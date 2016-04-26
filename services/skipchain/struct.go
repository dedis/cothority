package skipchain

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
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
}

type SkipBlockID crypto.HashID

// SkipBlockFix represents the fixed part of a SkipBlock
type SkipBlockFix struct {
	Index uint32
	// Height of that SkipBlock
	Height uint32
	// For deterministic SkipChains at what height to stop:
	// - if negative: we will use random distribution to calculate the
	// height of each new block
	// - else: the max height determines the height of the next block
	MaximumHeight uint32
	// BackLink is a slice of hashes to previous SkipBlocks
	BackLink []SkipBlockID
	// VerifierId is a SkipBlock-protocol verifying new SkipBlocks
	VerifierId VerifierID
	// SkipBlockParent points to the SkipBlock of the responsible Roster -
	// is nil if this is the Root-roster
	ParentBlock SkipBlockID
}

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

// SkipBlockCommon represents a SkipBlock
type SkipBlockCommon struct {
	*SkipBlockFix
	// Hash is calculated on all previous values
	Hash SkipBlockID
	// the signature on the above hash
	Signature *cosi.Signature
	// ForwardLink will be calculated once future SkipBlocks are
	// available
	ForwardLink []BlockLink
}

func NewSkipBlockCommon() *SkipBlockCommon {
	return &SkipBlockCommon{
		SkipBlockFix: &SkipBlockFix{},
		Signature:    cosi.NewSignature(network.Suite),
	}
}

func (sbc *SkipBlockCommon) VerifySignatures() error {
	return nil
}

func (sbc *SkipBlockCommon) GetCommon() *SkipBlockCommon {
	return sbc
}

type SkipBlockData struct {
	*SkipBlockCommon
	// Data is any data to b-e stored in that SkipBlock
	Data []byte
}

func (sbd *SkipBlockData) updateHash() SkipBlockID {
	sbd.Hash = sbd.addSliceToHash(sbd.Data)
	return sbd.Hash
}

type SkipBlockRoster struct {
	*SkipBlockCommon
	// EntityList holds the roster-definition of that SkipBlock
	EntityList *sda.EntityList
	// SkipLists that depend on us, given as the first SkipBlock - can
	// be a Data or a Roster SkipBlock
	ChildSL *BlockLink
}

func NewSkipBlockRoster(el *sda.EntityList) *SkipBlockRoster {
	return &SkipBlockRoster{
		SkipBlockCommon: NewSkipBlockCommon(),
		EntityList:      el,
		ChildSL: &BlockLink{
			Signature: cosi.NewSignature(network.Suite),
		},
	}
}

func (sbr *SkipBlockRoster) updateHash() SkipBlockID {
	sbr.Hash = sbr.addSliceToHash(sbr.EntityList.Id[:])
	return sbr.Hash
}

// BlockLink has the hash and a signature of a block
type BlockLink struct {
	Hash SkipBlockID
	*cosi.Signature
}
