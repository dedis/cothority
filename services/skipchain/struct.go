package skipchain

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

type SkipBlock interface {
	// Hash calculates the hash, writes it to the SkipBlock and returns
	// calculated hash.
	updateHash() SkipBlockID
	// VerifySignature checks if the main signature and all forward-links
	// are correctly signed and returns an error if not.
	VerifySignatures() error
	// GetCommon returns the part of the main information about the
	// SkipBlock which is the SkipBlockCommon structure.
	GetCommon() *SkipBlockCommon
}

type SkipBlockID crypto.HashID

// SkipBlock represents a skipblock
type SkipBlockCommon struct {
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
	// Hash is calculated on all previous values
	Hash SkipBlockID
	// the signature on the above hash
	Signature *cosi.Signature
	// ForwardLink will be calculated once future SkipBlocks are
	// available
	ForwardLink []BlockLink
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
	suite := network.Suite
	copy := *sbd
	copy.Signature = cosi.NewSignature(suite)
	copy.Hash = nil
	copy.ForwardLink = nil
	b, err := network.MarshalRegisteredType(&copy)
	if err != nil {
		dbg.Panic("Couldn't marshal skip-block:", err)
	}
	h, err := crypto.HashBytes(suite.Hash(), b)
	if err != nil {
		dbg.Panic("Couldn't hash skip-block:", err)
	}
	// store the generated hash:
	sbd.Hash = h
	return h
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
	suite := network.Suite
	copy := *sbr
	copy.Signature = cosi.NewSignature(suite)
	copy.Hash = nil
	copy.ForwardLink = nil
	b, err := network.MarshalRegisteredType(&copy)
	if err != nil {
		dbg.Panic("Couldn't marshal skip-block:", err)
	}
	h, err := crypto.HashBytes(suite.Hash(), b)
	if err != nil {
		dbg.Panic("Couldn't hash skip-block:", err)
	}
	// store the generated hash:
	sbr.Hash = h
	return h
}

func NewSkipBlockCommon() *SkipBlockCommon {
	return &SkipBlockCommon{
		Signature: cosi.NewSignature(network.Suite),
	}
}

// BlockLink has the hash and a signature of a block
type BlockLink struct {
	Hash SkipBlockID
	*cosi.Signature
}
