package skipchain

import (
	"bytes"

	"fmt"

	"sync"

	"errors"

	"strings"

	"github.com/satori/go.uuid"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/cosi"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// How many msec to wait before a timeout is generated in the propagation.
const propagateTimeout = 10000

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

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func RegisterVerification(c *onet.Context, v VerifierID, f SkipBlockVerifier) error {
	scs := c.Service(ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + ServiceName)
	}
	return scs.(*Service).registerVerification(v, f)
}

var (
	// VerifyShard makes sure that the child SkipChain will always be
	// a part of its parent SkipChain
	VerifyShard = VerifierID(uuid.NewV5(uuid.NamespaceURL, "Shard"))
	// VerifyBase checks that the base-parameters are correct, i.e.,
	// the links are correctly set up, the height-parameters and the
	// verification didn't change.
	VerifyBase = VerifierID(uuid.NewV5(uuid.NamespaceURL, "Base"))
)

var VerificationStandard = []VerifierID{VerifyBase}
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
		if err := fl.VerifySignature(sb.Roster.Publics()); err != nil {
			return errors.New("Wrong signature in forward-link: " + err.Error())
		}
	}
	return nil
}

// VerifyLinks makes sure that all forward- and backward-links are correct.
// It needs a SkipBlockMap to fetch other necessary blocks.
func (sb *SkipBlock) VerifyLinks(sbm *SkipBlockMap) error {
	if len(sb.BackLinkIDs) == 0 {
		return errors.New("need at least one backlink")
	}

	if err := sb.VerifyForwardSignatures(); err != nil {
		return errors.New("Wrong signatures: " + err.Error())
	}

	// Verify if we're in the responsible-list
	if !sb.ParentBlockID.IsNull() {
		parent, ok := sbm.GetByID(sb.ParentBlockID)
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
	for _, back := range sb.BackLinkIDs {
		sbBack, ok := sbm.GetByID(back)
		if !ok {
			return errors.New("didn't find skipblock in sbm")
		}
		if err := sbBack.VerifyForwardSignatures(); err != nil {
			return err
		}
		found := false
		for _, forward := range sbBack.ForwardLink {
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
	b.VerifierIDs = make([]VerifierID, len(sb.VerifierIDs))
	copy(b.VerifierIDs, sb.VerifierIDs)
	return &b
}

func (sb *SkipBlock) String() string {
	return sb.Hash.String()
}

// GetResponsible searches for the block that is responsible for us - for
// - Root_Genesis - himself
// - *_Gensis - it's his parent
// - else - it's the previous block
func (sb *SkipBlock) GetResponsible(sbm *SkipBlockMap) (*SkipBlock, error) {
	if sb == nil {
		log.Panic(log.Stack())
	}
	if sb.Index == 0 {
		// Genesis-block
		if sb.ParentBlockID.IsNull() {
			// Root-skipchain, no other parent
			return sb, nil
		}
		ret, ok := sbm.GetByID(sb.ParentBlockID)
		if !ok {
			return nil, errors.New("No Roster and no parent")
		}
		return ret, nil
	}
	if len(sb.BackLinkIDs) == 0 {
		return nil, errors.New("Invalid block: no backlink")
	}
	prev, ok := sbm.GetByID(sb.BackLinkIDs[0])
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

// SkipBlockMap holds the map to the skipblocks. This is used for verification,
// so that all links can be followed.
type SkipBlockMap struct {
	SkipBlocks map[string]*SkipBlock
	sync.Mutex
}

// NewSkipBlockMap returns a pre-initialised SkipBlockMap.
func NewSkipBlockMap() *SkipBlockMap {
	return &SkipBlockMap{SkipBlocks: make(map[string]*SkipBlock)}
}

// GetByID returns the skip-block or false if it doesn't exist
func (s *SkipBlockMap) GetByID(sbID SkipBlockID) (*SkipBlock, bool) {
	s.Lock()
	b, ok := s.SkipBlocks[string(sbID)]
	s.Unlock()
	return b, ok
}

// Store stores the given SkipBlock in the service-list
func (s *SkipBlockMap) Store(sb *SkipBlock) SkipBlockID {
	s.Lock()
	if sbOld, exists := s.SkipBlocks[string(sb.Hash)]; exists {
		// If this skipblock already exists, only copy forward-links.
		for _, fl := range sb.ForwardLink[len(sbOld.ForwardLink):] {
			if err := fl.VerifySignature(sbOld.Roster.Publics()); err != nil {
				log.Error("Got a known block with wrong signature in forward-link")
				return nil
			}
			sbOld.ForwardLink = append(sbOld.ForwardLink, fl)
		}
	} else {
		s.SkipBlocks[string(sb.Hash)] = sb
	}
	s.Unlock()
	return sb.Hash
}

// Length returns the actual length using mutexes
func (s *SkipBlockMap) Length() int {
	s.Lock()
	defer s.Unlock()
	return len(s.SkipBlocks)
}

// StringMaps returns a string of all map-keys
func (s *SkipBlockMap) StringMaps() string {
	var str []string
	for k := range s.SkipBlocks {
		str = append(str, fmt.Sprintf("%8x", k))
	}
	return strings.Join(str, "\n")
}

// VerifyLink searches for a link between two IDs. The second ID must
// be reachable through forward-links from the first ID. The return-value
// can be:
//  0 - All forward-links are correct
//  1 - Not enough forward-links - first and second exist, but no link is found
//  2 - Some error occurred: either wrong signatures, other block-id, or other
func (s *SkipBlockMap) VerifyLink(src, dst SkipBlockID) (result int) {
	result = 2
	sbSrc, exists := s.GetByID(src)
	if !exists {
		return
	}
	sbDst, exists := s.GetByID(dst)
	if !exists {
		return
	}
	log.Print(sbSrc, sbDst)
	return 0
}
