package skipchain

import (
	"bytes"

	"fmt"

	"errors"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/bftcosi"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/cosi"
)

// How many msec to wait before a timeout is generated in the propagation.
const propagateTimeout = 10000

// AppSkipBlock is the interface needed to add a new SkipBlockType with
// a verification
type AppSkipBlock interface {
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

// Equal compares the hash of the two skipblocks
func (sbid SkipBlockID) Equal(sb SkipBlockID) bool {
	return bytes.Equal([]byte(sbid), []byte(sb))
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
	// Roster holds the roster-definition of that SkipBlock
	Roster *sda.Roster
}

// addSliceToHash hashes the whole SkipBlockFix plus a slice of bytes.
// This is used
func (sbf *SkipBlockFix) calculateHash() SkipBlockID {
	b, err := network.MarshalRegisteredType(sbf)
	if err != nil {
		log.Panic("Couldn't marshal SkipBlockFix:", err)
	}
	h, err := crypto.HashBytes(network.Suite.Hash(), b)
	if err != nil {
		log.Panic("Couldn't hash SkipBlockFix:", err)
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
			Sig: make([]byte, 0),
			Msg: make([]byte, 0),
		},
	}
}

// VerifySignatures returns whether all signatures are correctly signed
// by the aggregate public key of the roster. It needs the aggregate key.
func (sb *SkipBlock) VerifySignatures() error {
	if err := sb.BlockSig.Verify(network.Suite, sb.Roster.Publics()); err != nil {
		log.Error(err.Error() + log.Stack())
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
	sigCopy := make([]byte, len(b.BlockSig.Sig))
	copy(sigCopy, b.BlockSig.Sig)
	b.BlockSig = &bftcosi.BFTSignature{
		Sig:        sigCopy,
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
// - Data - it's his parent
// - else - it's himself
func (sb *SkipBlock) GetResponsible(s *Service) (*sda.Roster, error) {
	el := sb.Roster
	if el == nil {
		// We're a data-block, so use the parent's Roster
		if sb.ParentBlockID.IsNull() {
			return nil, errors.New("Didn't find an Roster")
		}
		parent, ok := s.getSkipBlockByID(sb.ParentBlockID)
		if !ok {
			return nil, errors.New("No Roster and no parent")
		}
		if parent.Roster == nil {
			return nil, errors.New("Parent doesn't have Roster")
		}
		el = parent.Roster
	}
	return el, nil
}

func (sb *SkipBlock) updateHash() SkipBlockID {
	sb.Hash = sb.calculateHash()
	return sb.Hash
}

// BlockLink has the hash and a signature of a block
type BlockLink struct {
	Hash      SkipBlockID
	Signature []byte
}

// NewBlockLink pre-initialises the signature so it can be sent
// over the network.
func NewBlockLink() *BlockLink {
	return &BlockLink{
		Signature: make([]byte, 0),
	}
}

// Copy makes a deep copy of a blocklink
func (bl *BlockLink) Copy() *BlockLink {
	sigCopy := make([]byte, len(bl.Signature))
	copy(sigCopy, bl.Signature)
	return &BlockLink{
		Hash:      bl.Hash,
		Signature: sigCopy,
	}
}

// VerifySignature returns whether the BlockLink has been signed
// correctly using the aggregate key given.
func (bl *BlockLink) VerifySignature(publics []abstract.Point) error {
	// TODO: enable real verification once we have signatures
	//return nil
	return cosi.VerifySignature(network.Suite, publics, bl.Hash, bl.Signature)
}
