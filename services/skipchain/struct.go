package skipchain

import (
	"bytes"

	"fmt"

	"errors"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/bftcosi"
	"github.com/dedis/crypto/abstract"
)

type AppSkipBlock interface{
	// VerifyNewSkipBlock takes the last signed SkipBlock and the
	// proposed Skipblock. It returns true if the proposed SkipBlock
	// has to be accepted or false if it rejects.
	VerifyProposedSkipBlock(last, proposed *SkipBlock) bool
	VoteNewSkipBlock(proposedID SkipBlockID, vote bool)
}

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
	// AggregateResp is the aggreate key of the responsible block for us
	AggregateResp abstract.Point
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
	// Hash is our Block-hash
	Hash SkipBlockID
	// BlockSig is the BFT-signature of the hash
	BlockSig *bftcosi.BFTSignature

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
		BlockSig: &bftcosi.BFTSignature{
			Sig: cosi.NewSignature(network.Suite),
			Msg: make([]byte, 0),
		},
	}
}

// VerifySignatures returns whether all signatures are correctly signed
// by the aggregate public key of the roster. It needs the aggregate key.
func (sb *SkipBlock) VerifySignatures() error {
	if err := sb.BlockSig.Verify(network.Suite, sb.AggregateResp, sb.Hash); err != nil {
		dbg.Error(err.Error() + dbg.Stack())
		return err
	}
	//for _, fl := range sb.ForwardLink {
	//	if err := fl.VerifySignature(sb.Aggregate); err != nil {
	//		return err
	//	}
	//}
	//if sb.ChildSL != nil && sb.ChildSL.Hash == nil {
	//	return sb.ChildSL.VerifySignature(sb.Aggregate)
	//}
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
	b.BlockSig = &bftcosi.BFTSignature{
		Sig:        &cosi.Signature{b.BlockSig.Sig.Challenge, b.BlockSig.Sig.Response},
		Msg:        b.BlockSig.Msg,
		Exceptions: b.BlockSig.Exceptions,
	}
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

// GetResponsible searches for the block that is responsible for us - for
// - Genesis-block - it's himself
// - Follower - it's the previous block
// - Data - it's his parent
func (sb *SkipBlock) GetResponsible(s *Service) (*sda.EntityList, error) {
	el := sb.EntityList
	if el == nil {
		// We're a data-block, so use the parent's EntityList
		if sb.ParentBlockID.IsNull() {
			return nil, errors.New("Didn't find an EntityList")
		}
		parent, ok := s.getSkipBlockByID(sb.ParentBlockID)
		if !ok {
			return nil, errors.New("No EntityList and no parent")
		}
		if parent.EntityList == nil {
			return nil, errors.New("Parent doesn't have EntityList")
		}
		el = parent.EntityList
	} else {
		if sb.Index > 0 {
			latest, ok := s.getSkipBlockByID(sb.BackLinkIds[0])
			if !ok {
				return nil, errors.New("Non-genesis block and no previous block available")
			}
			el = latest.EntityList
		}
	}
	return el, nil
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
	//return nil
	return cosi.VerifySignature(network.Suite, bl.Hash, aggregate,
		bl.Challenge, bl.Response)
}
