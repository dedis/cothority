package skipchain

import (
	"bytes"

	"fmt"

	"sync"

	"errors"

	"encoding/binary"

	"encoding/hex"

	"github.com/satori/go.uuid"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/cosi"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	for _, m := range []interface{}{
		// - Data structures
		&SkipBlockFix{},
		&SkipBlock{},
	} {
		network.RegisterMessage(m)
	}

}

// SkipBlockID represents the Hash of the SkipBlock
type SkipBlockID []byte

// IsNull returns true if the ID is undefined
func (sbid SkipBlockID) IsNull() bool {
	return len(sbid) == 0
}

// Short returns only the 8 first bytes of the ID as a hex-encoded string.
func (sbid SkipBlockID) Short() string {
	if sbid.IsNull() {
		return "Nil"
	}
	return fmt.Sprintf("%x", []byte(sbid[0:8]))
}

// Equal compares the hash of the two skipblocks
func (sbid SkipBlockID) Equal(sb SkipBlockID) bool {
	return bytes.Equal([]byte(sbid), []byte(sb))
}

// VerifierID represents one of the verifications used to accept or
// deny a SkipBlock.
type VerifierID uuid.UUID

// SkipBlockVerifier is function that should return whether this skipblock is
// accepted or not. This function is used during a BFTCosi round, but wrapped
// around so it accepts a block.
//
//   newID is the hash of the new block that will be signed
//   newSB is the new block
type SkipBlockVerifier func(newID []byte, newSB *SkipBlock) bool

var (
	// VerifyBase checks that the base-parameters are correct, i.e.,
	// the links are correctly set up, the height-parameters and the
	// verification didn't change.
	VerifyBase = VerifierID(uuid.NewV5(uuid.NamespaceURL, "Base"))
)

// VerificationStandard makes sure that all links are correct and that the
// basic parameters like height, GenesisID and others don't change between
// blocks.
var VerificationStandard = []VerifierID{VerifyBase}

// VerificationNone is mostly used for test - it allows for nearly every new
// block to be appended.
var VerificationNone = []VerifierID{}

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
	// GenesisID is the ID of the genesis-block.
	GenesisID SkipBlockID
	// Data is any data to be stored in that SkipBlock
	Data []byte
	// Roster holds the roster-definition of that SkipBlock
	Roster *onet.Roster
}

// SkipBlockData represents all entries - as maps are not ordered and thus
// difficult to hash, this is as a slice to {key,data}-pairs.
type SkipBlockData struct {
	Entries []SkipBlockDataEntry
}

// Get returns the data-portion of the key. If key does not exist, it returns
// nil.
func (sbd *SkipBlockData) Get(key string) []byte {
	for _, d := range sbd.Entries {
		if d.Key == key {
			return d.Data
		}
	}
	return nil
}

// Set replaces an existing entry or adds a new entry if the key is not
// existant.
func (sbd *SkipBlockData) Set(key string, data []byte) {
	for i := range sbd.Entries {
		if sbd.Entries[i].Key == key {
			sbd.Entries[i].Data = data
			return
		}
	}
	sbd.Entries = append(sbd.Entries, SkipBlockDataEntry{key, data})
}

// SkipBlockDataEntry is one entry for the SkipBlockData.
type SkipBlockDataEntry struct {
	Key  string
	Data []byte
}

// addSliceToHash hashes the whole SkipBlockFix plus a slice of bytes.
// This is used
func (sbf *SkipBlockFix) CalculateHash() SkipBlockID {
	hash := network.Suite.Hash()
	for _, i := range []int{sbf.Index, sbf.Height, sbf.MaximumHeight,
		sbf.BaseHeight} {
		binary.Write(hash, binary.LittleEndian, i)
	}
	for _, bl := range sbf.BackLinkIDs {
		hash.Write(bl)
	}
	for _, v := range sbf.VerifierIDs {
		hash.Write(v[:])
	}
	hash.Write(sbf.ParentBlockID)
	hash.Write(sbf.GenesisID)
	hash.Write(sbf.Data)
	if sbf.Roster != nil {
		for _, pub := range sbf.Roster.Publics() {
			pub.MarshalTo(hash)
		}
	}
	buf := hash.Sum(nil)
	return buf
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
	fwMutex     sync.Mutex
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
	sb.fwMutex.Lock()
	defer sb.fwMutex.Unlock()
	for _, fl := range sb.ForwardLink {
		if err := fl.VerifySignature(sb.Roster.Publics()); err != nil {
			return errors.New("Wrong signature in forward-link: " + err.Error())
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
	sb.fwMutex.Lock()
	sbf := *sb.SkipBlockFix
	b := &SkipBlock{
		SkipBlockFix: &sbf,
		Hash:         make([]byte, len(sb.Hash)),
		ForwardLink:  make([]*BlockLink, len(sb.ForwardLink)),
		ChildSL:      make([]SkipBlockID, len(sb.ChildSL)),
	}
	for i, fl := range sb.ForwardLink {
		b.ForwardLink[i] = fl.Copy()
	}
	copy(b.ChildSL, sb.ChildSL)
	copy(b.Hash, sb.Hash)
	b.VerifierIDs = make([]VerifierID, len(sb.VerifierIDs))
	copy(b.VerifierIDs, sb.VerifierIDs)
	sb.fwMutex.Unlock()
	return b
}

// Short returns only the 8 first bytes of the hash as hex-encoded string.
func (sb *SkipBlock) Short() string {
	return sb.Hash.Short()
}

// Sprint returns a string describing that block. If 'short' is true, it will
// only return the first 8 bytes of the genesis and its own id.
func (sb *SkipBlock) Sprint(short bool) string {
	hash := hex.EncodeToString(sb.Hash)
	if short {
		hash = hash[:8]
	}
	if sb.Index == 0 {
		return fmt.Sprintf("Genesis-block %s with roster %s",
			hash, sb.Roster.List)
	}
	return fmt.Sprintf("Block %s and roster %s",
		hash, sb.Roster.List)
}

// SkipChainID is the hash of the genesis-block.
func (sb *SkipBlock) SkipChainID() SkipBlockID {
	if sb.Index == 0 {
		return sb.Hash
	}
	return sb.GenesisID
}

// AddForward stores the forward-link with mutex protection.
func (sb *SkipBlock) AddForward(fw *BlockLink) {
	sb.fwMutex.Lock()
	sb.ForwardLink = append(sb.ForwardLink, fw)
	sb.fwMutex.Unlock()
}

// GetForward returns copy of the forward-link at position i. It returns nil if no link
// at that level exists.
func (sb *SkipBlock) GetForward(i int) *BlockLink {
	sb.fwMutex.Lock()
	defer sb.fwMutex.Unlock()
	if len(sb.ForwardLink) <= i {
		return nil
	}
	return sb.ForwardLink[i].Copy()
}

// GetForwardLen returns the number of ForwardLinks.
func (sb *SkipBlock) GetForwardLen() int {
	sb.fwMutex.Lock()
	defer sb.fwMutex.Unlock()
	return len(sb.ForwardLink)
}

func (sb *SkipBlock) UpdateHash() SkipBlockID {
	sb.Hash = sb.CalculateHash()
	return sb.Hash
}

// BlockLink has the hash and a signature of a block
type BlockLink struct {
	Hash      SkipBlockID
	Signature []byte
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
// correctly using the given list of public keys.
func (bl *BlockLink) VerifySignature(publics []abstract.Point) error {
	if len(bl.Signature) == 0 {
		return errors.New("No signature present" + log.Stack())
	}
	return cosi.VerifySignature(network.Suite, publics, bl.Hash, bl.Signature)
}
