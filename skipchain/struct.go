package skipchain

import (
	"bytes"

	"fmt"

	"sync"

	"errors"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/cosi"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
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
type SkipBlockID []byte

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
	BackLinkIds []SkipBlockID
	// VerifierID is a SkipBlock-protocol verifying new SkipBlocks
	VerifierID VerifierID
	// SkipBlockParent points to the SkipBlock of the responsible Roster -
	// is nil if this is the Root-roster
	ParentBlockID SkipBlockID
	// GenesisID is the ID of the genesis-block.
	GenesisID SkipBlockID
	// RespPublic is the list of public keys of our responsible
	RespPublic []abstract.Point
	// Data is any data to be stored in that SkipBlock
	Data []byte
	// Roster holds the roster-definition of that SkipBlock
	Roster *onet.Roster
}

// addSliceToHash hashes the whole SkipBlockFix plus a slice of bytes.
// This is used
func (sbf *SkipBlockFix) calculateHash() SkipBlockID {
	b, err := network.Marshal(sbf)
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

	// ForwardLink will be calculated once future SkipBlocks are
	// available
	ForwardLink []*BlockLink
	// SkipLists that depend on us, given as the first SkipBlock - can
	// be a Data or a Roster SkipBlock
	ChildSL []SkipBlockID
}

// NewSkipBlock pre-initialises the block so it can be sent over
// the network
func NewSkipBlock() *SkipBlock {
	return &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			Data: make([]byte, 0),
		},
	}
}

// VerifyForwardSignatures returns whether all signatures in the forward-links
// are correctly signed by the aggregate public key of the roster.
func (sb *SkipBlock) VerifyForwardSignatures() error {
	for _, fl := range sb.ForwardLink {
		if err := fl.VerifySignature(sb.RespPublic); err != nil {
			return errors.New("Wrong signature in forward-link: " + err.Error())
		}
	}
	return nil
}

// VerifyLinks makes sure that all forward- and backward-links are correct.
// It needs a SkipBlockMap to fetch other necessary blocks.
func (sb *SkipBlock) VerifyLinks(sbm *SkipBlockMap) error {
	if len(sb.BackLinkIds) == 0 {
		return errors.New("need at least one backlink")
	}

	if err := sb.VerifyForwardSignatures(); err != nil {
		return errors.New("Wrong signatures: " + err.Error())
	}

	// Verify if we're in the responsible-list
	if !sb.ParentBlockID.IsNull() {
		parent, ok := sbm.GetSkipBlockByID(sb.ParentBlockID)
		if !ok {
			return errors.New("Didn't find parent")
		}
		if err := parent.VerifyForwardSignatures(); err != nil {
			return err
		}
		found := false
		for _, child := range parent.ChildSL {
			if child.Equal(sb.Hash) {
				found = true
				break
			}
		}
		if !found {
			return errors.New("parent doesn't know about us")
		}
	}

	// We don't check backward-links for genesis-blocks
	if sb.Index == 0 {
		return nil
	}
	for _, back := range sb.BackLinkIds {
		sbBack, ok := sbm.GetSkipBlockByID(back)
		if !ok {
			return errors.New("didn't find skipblock in sbm")
		}
		if err := sbBack.VerifyForwardSignatures(); err != nil {
			return err
		}
		found := false
		for _, forward := range sb.ForwardLink {
			if forward.Hash.Equal(sb.Hash) {
				found = true
				break
			}
		}
		if !found {
			return errors.New("didn't find our block in forward-links")
		}
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
	b.ForwardLink = make([]*BlockLink, len(sb.ForwardLink))
	for i, fl := range sb.ForwardLink {
		b.ForwardLink[i] = fl.Copy()
	}
	b.ChildSL = make([]SkipBlockID, len(sb.ChildSL))
	copy(b.ChildSL, sb.ChildSL)
	return &b
}

func (sb *SkipBlock) String() string {
	return sb.Hash.String()
}

// GetResponsible searches for the block that is responsible for us - for
// - Root_Genesis - himself
// - Data || Inter_Gensis - it's his parent
// - else - it's the previous block
func (sb *SkipBlock) GetResponsible(s *SkipBlockMap) (*SkipBlock, error) {
	if sb == nil {
		log.Panic(log.Stack())
	}
	if sb.Index == 0 && sb.ParentBlockID.IsNull() {
		return sb, nil
	}
	if sb.Roster == nil || sb.Index == 0 {
		// We're a data-block, so use the parent's Roster
		if sb.ParentBlockID.IsNull() {
			return nil, errors.New("Didn't find a Roster")
		}
		ret, ok := s.GetSkipBlockByID(sb.ParentBlockID)
		if !ok {
			return nil, errors.New("No Roster and no parent")
		}
		return ret, nil
	}
	if len(sb.BackLinkIds) == 0 {
		return nil, errors.New("No backlink for non-genesis block")
	}
	prev, ok := s.GetSkipBlockByID(sb.BackLinkIds[0])
	if !ok {
		return nil, errors.New("Didn't find responsible")
	}
	return prev, nil
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
	if len(bl.Signature) == 0 {
		return errors.New("No signature present" + log.Stack())
	}
	return cosi.VerifySignature(network.Suite, publics, bl.Hash, bl.Signature)
}

// SkipBlockMap holds the map to the skipblocks so it can be marshaled.
type SkipBlockMap struct {
	SkipBlocks map[string]*SkipBlock
	sync.Mutex
}

// NewSkipBlockMap returns a pre-initialised SkipBlockMap
func NewSkipBlockMap() *SkipBlockMap {
	return &SkipBlockMap{SkipBlocks: make(map[string]*SkipBlock)}
}

// GetSkipBlockByID returns the skip-block or false if it doesn't exist
func (s *SkipBlockMap) GetSkipBlockByID(sbID SkipBlockID) (*SkipBlock, bool) {
	s.Lock()
	b, ok := s.SkipBlocks[string(sbID)]
	s.Unlock()
	return b, ok
}

// StoreSkipBlock stores the given SkipBlock in the service-list
func (s *SkipBlockMap) StoreSkipBlock(sb *SkipBlock) SkipBlockID {
	s.Lock()
	s.SkipBlocks[string(sb.Hash)] = sb
	s.Unlock()
	return sb.Hash
}

// LenSkipBlock returns the actual length using mutexes
func (s *SkipBlockMap) LenSkipBlocks() int {
	s.Lock()
	defer s.Unlock()
	return len(s.SkipBlocks)
}
