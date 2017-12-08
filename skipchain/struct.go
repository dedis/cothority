package skipchain

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/cosi/crypto"
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/satori/go.uuid"
)

// How many msec to wait before a timeout is generated in the propagation.
const propagateTimeout = 10000

// How often we save the skipchains - in seconds.
const timeBetweenSave = 0

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

// String returns canonical string representation of the ID
func (vId VerifierID) String() string {
	return uuid.UUID(vId).String()
}

// Equal returns true if and only if vID2 equals this VerifierID.
func (vId VerifierID) Equal(vID2 VerifierID) bool {
	return uuid.Equal(uuid.UUID(vId), uuid.UUID(vID2))
}

// IsNil returns true iff the VerifierID is Nil
func (vId VerifierID) IsNil() bool {
	return vId.Equal(VerifierID(uuid.Nil))
}

// SkipBlockVerifier is function that should return whether this skipblock is
// accepted or not. This function is used during a BFTCosi round, but wrapped
// around so it accepts a block.
//
//   newID is the hash of the new block that will be signed
//   newSB is the new block
type SkipBlockVerifier func(newID []byte, newSB *SkipBlock) bool

// GetService makes it possible to give either an `onet.Context` or
// `onet.Server` to `RegisterVerification`.
type GetService interface {
	Service(name string) onet.Service
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func RegisterVerification(s GetService, v VerifierID, f SkipBlockVerifier) error {
	scs := s.Service(ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + ServiceName)
	}
	return scs.(*Service).registerVerification(v, f)
}

var (
	// VerifyBase checks that the base-parameters are correct, i.e.,
	// the links are correctly set up, the height-parameters and the
	// verification didn't change.
	VerifyBase = VerifierID(uuid.NewV5(uuid.NamespaceURL, "Base"))
	// VerifyRoot depends on a data-block being a slice of public keys
	// that are used to sign the next block. The private part of those
	// keys are supposed to be offline. It makes sure
	// that every new block is signed by the keys present in the previous block.
	VerifyRoot = VerifierID(uuid.NewV5(uuid.NamespaceURL, "Root"))
	// VerifyControl makes sure this chain is a child of a Root-chain and
	// that there is now new block if a newer parent is present.
	// It also makes sure that no more than 1/3 of the members of the roster
	// change between two blocks.
	VerifyControl = VerifierID(uuid.NewV5(uuid.NamespaceURL, "Control"))
	// VerifyData makes sure that:
	//   - it has a parent-chain with `VerificationControl`
	//   - its Roster doesn't change between blocks
	//   - if there is a newer parent, no new block will be appended to that chain.
	VerifyData = VerifierID(uuid.NewV5(uuid.NamespaceURL, "Data"))
)

// VerificationStandard makes sure that all links are correct and that the
// basic parameters like height, GenesisID and others don't change between
// blocks.
var VerificationStandard = []VerifierID{VerifyBase}

// VerificationRoot is used to create a root-chain that has 'Control'-chains
// as its children.
var VerificationRoot = []VerifierID{VerifyBase, VerifyRoot}

// VerificationControl is used in chains that depend on a 'Root'-chain.
var VerificationControl = []VerifierID{VerifyBase, VerifyControl}

// VerificationData is used in chains that depend on a 'Control'-chain.
var VerificationData = []VerifierID{VerifyBase, VerifyData}

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

// CalculateHash hashes all fixed fields of the skipblock.
func (sbf *SkipBlockFix) CalculateHash() SkipBlockID {
	hash := cothority.Suite.Hash()
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

// Equal returns bool if both hashes are equal
func (sb *SkipBlock) Equal(other *SkipBlock) bool {
	return bytes.Equal(sb.Hash, other.Hash)
}

// Copy makes a deep copy of the SkipBlock
func (sb *SkipBlock) Copy() *SkipBlock {
	if sb == nil {
		return nil
	}
	sbf := *sb.SkipBlockFix
	sbf.BackLinkIDs = make([]SkipBlockID, len(sb.SkipBlockFix.BackLinkIDs))
	for i := range sbf.BackLinkIDs {
		sbf.BackLinkIDs[i] = make(SkipBlockID, len(sb.SkipBlockFix.BackLinkIDs[i]))
		copy(sbf.BackLinkIDs[i], sb.SkipBlockFix.BackLinkIDs[i])
	}
	sbf.VerifierIDs = make([]VerifierID, len(sb.SkipBlockFix.VerifierIDs))
	for i := range sbf.VerifierIDs {
		sbf.VerifierIDs[i] = sb.SkipBlockFix.VerifierIDs[i]
	}

	sbf.Data = make([]byte, len(sb.SkipBlockFix.Data))
	copy(sbf.Data, sb.SkipBlockFix.Data)

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
	sb.ForwardLink = append(sb.ForwardLink, fw)
}

// GetForward returns copy of the forward-link at position i. It returns nil if no link
// at that level exists.
func (sb *SkipBlock) GetForward(i int) *BlockLink {
	if len(sb.ForwardLink) <= i {
		return nil
	}
	return sb.ForwardLink[i].Copy()
}

// GetForwardLen returns the number of ForwardLinks.
func (sb *SkipBlock) GetForwardLen() int {
	return len(sb.ForwardLink)
}

func (sb *SkipBlock) updateHash() SkipBlockID {
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
	hashCopy := make([]byte, len(bl.Hash))
	copy(hashCopy, bl.Hash)
	return &BlockLink{
		Hash:      hashCopy,
		Signature: sigCopy,
	}
}

// VerifySignature returns whether the BlockLink has been signed
// correctly using the given list of public keys.
func (bl *BlockLink) VerifySignature(publics []kyber.Point) error {
	if len(bl.Signature) == 0 {
		return errors.New("No signature present" + log.Stack())
	}
	return crypto.VerifySignature(cothority.Suite, publics, bl.Hash, bl.Signature)
}

// SkipBlockMap holds the map to the skipblocks. This is used for verification,
// so that all links can be followed.
type SkipBlockMap struct {
	SkipBlocks map[string]*SkipBlock
	sync.Mutex
}

// SkipBlockDB holds the database to the skipblocks.
// This is used for verification, so that all links can be followed.
// It is a wrapper to embed bolt.DB.
type SkipBlockDB struct {
	*bolt.DB
}

// NewSkipBlockMap returns a pre-initialised SkipBlockMap.
func NewSkipBlockMap() *SkipBlockMap {
	return &SkipBlockMap{SkipBlocks: make(map[string]*SkipBlock)}
}

// GetByID returns the skip-block or nil if it doesn't exist
func (sbm *SkipBlockMap) GetByID(sbID SkipBlockID) *SkipBlock {
	sbm.Lock()
	defer sbm.Unlock()
	return sbm.SkipBlocks[string(sbID)].Copy()
}

// GetByID returns a new copy of the skip-block or nil if it doesn't exist
func (db *SkipBlockDB) GetByID(sbID SkipBlockID) *SkipBlock {
	sb, err := db.dbGet(sbID)
	if err != nil {
		log.Error(err.Error())
	}
	return sb
}

// Store stores the given SkipBlock in the service-list
func (sbm *SkipBlockMap) Store(sb *SkipBlock) SkipBlockID {
	sbm.Lock()
	defer sbm.Unlock()
	if sbOld, exists := sbm.SkipBlocks[string(sb.Hash)]; exists {
		// If this skipblock already exists, only copy forward-links and
		// new children.
		if len(sb.ForwardLink) > len(sbOld.ForwardLink) {
			for _, fl := range sb.ForwardLink[len(sbOld.ForwardLink):] {
				if err := fl.VerifySignature(sbOld.Roster.Publics()); err != nil {
					log.Error("Got a known block with wrong signature in forward-link")
					return nil
				}
				sbOld.ForwardLink = append(sbOld.ForwardLink, fl)
			}
		}
		if len(sb.ChildSL) > len(sbOld.ChildSL) {
			sbOld.ChildSL = append(sbOld.ChildSL, sb.ChildSL[len(sbOld.ChildSL):]...)
		}
	} else {
		sbm.SkipBlocks[string(sb.Hash)] = sb
	}
	return sb.Hash
}

// Store stores the given SkipBlock in the service-list
func (db *SkipBlockDB) Store(sb *SkipBlock) SkipBlockID {
	sbOld, err := db.dbGet(sb.Hash)
	if err != nil {
		log.Error("failed to get skipblock with error: " + err.Error())
		return nil
	}
	if sbOld != nil {
		// If this skipblock already exists, only copy forward-links and
		// new children.
		if len(sb.ForwardLink) > len(sbOld.ForwardLink) {
			for _, fl := range sb.ForwardLink[len(sbOld.ForwardLink):] {
				if err := fl.VerifySignature(sbOld.Roster.Publics()); err != nil {
					log.Error("Got a known block with wrong signature in forward-link")
					return nil
				}
				sbOld.ForwardLink = append(sbOld.ForwardLink, fl)
			}
		}
		if len(sb.ChildSL) > len(sbOld.ChildSL) {
			sbOld.ChildSL = append(sbOld.ChildSL, sb.ChildSL[len(sbOld.ChildSL):]...)
		}
		err := db.dbStore(sbOld)
		if err != nil {
			log.Error(err.Error())
		}
	} else {
		err := db.dbStore(sb)
		if err != nil {
			log.Error(err.Error())
		}
	}
	return sb.Hash
}

// Length returns the actual length using mutexes
func (sbm *SkipBlockMap) Length() int {
	sbm.Lock()
	defer sbm.Unlock()
	return len(sbm.SkipBlocks)
}

// Length returns the actual length using mutexes
func (db *SkipBlockDB) Length() int {
	var i int
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(skipblocksBucket))
		i = b.Stats().KeyN
		return nil
	})
	return i
}

// GetResponsible searches for the block that is responsible for sb
// - Root_Genesis - himself
// - *_Gensis - it's his parent
// - else - it's the previous block
func (sbm *SkipBlockMap) GetResponsible(sb *SkipBlock) (*SkipBlock, error) {
	if sb == nil {
		log.Panic(log.Stack())
	}
	if sb.Index == 0 {
		// Genesis-block
		if sb.ParentBlockID.IsNull() {
			// Root-skipchain, no other parent
			return sb, nil
		}
		ret := sbm.GetByID(sb.ParentBlockID)
		if ret == nil {
			return nil, errors.New("No Roster and no parent")
		}
		return ret, nil
	}
	if len(sb.BackLinkIDs) == 0 {
		return nil, errors.New("Invalid block: no backlink")
	}
	prev := sbm.GetByID(sb.BackLinkIDs[0])
	if prev == nil {
		return nil, errors.New("Didn't find responsible")
	}
	return prev, nil
}

// GetResponsible searches for the block that is responsible for sb
// - Root_Genesis - himself
// - *_Gensis - it's his parent
// - else - it's the previous block
func (db *SkipBlockDB) GetResponsible(sb *SkipBlock) (*SkipBlock, error) {
	if sb == nil {
		log.Panic(log.Stack())
	}
	if sb.Index == 0 {
		// Genesis-block
		if sb.ParentBlockID.IsNull() {
			// Root-skipchain, no other parent
			return sb, nil
		}
		ret := db.GetByID(sb.ParentBlockID)
		if ret == nil {
			return nil, errors.New("No Roster and no parent")
		}
		return ret, nil
	}
	if len(sb.BackLinkIDs) == 0 {
		return nil, errors.New("Invalid block: no backlink")
	}
	prev := db.GetByID(sb.BackLinkIDs[0])
	if prev == nil {
		return nil, errors.New("Didn't find responsible")
	}
	return prev, nil
}

// VerifyLinks makes sure that all forward- and backward-links are correct.
// It takes a skipblock to verify and returns nil in case of success.
func (sbm *SkipBlockMap) VerifyLinks(sb *SkipBlock) error {
	if len(sb.BackLinkIDs) == 0 {
		return errors.New("need at least one backlink")
	}

	if err := sb.VerifyForwardSignatures(); err != nil {
		return errors.New("Wrong signatures: " + err.Error())
	}

	// Verify if we're in the responsible-list
	if !sb.ParentBlockID.IsNull() {
		parent := sbm.GetByID(sb.ParentBlockID)
		if parent == nil {
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

	// Verify we're referenced by our previous block
	sbBack := sbm.GetByID(sb.BackLinkIDs[0])
	if sbBack == nil {
		if sb.GetForwardLen() > 0 {
			log.Lvl3("Didn't find back-link, but have a good forward-link")
			return nil
		}
		return errors.New("Didn't find height-0 skipblock in sbm")
	}
	if err := sbBack.VerifyForwardSignatures(); err != nil {
		return err
	}
	if !sbBack.GetForward(0).Hash.Equal(sb.Hash) {
		return errors.New("didn't find our block in forward-links")
	}
	return nil
}

// VerifyLinks makes sure that all forward- and backward-links are correct.
// It takes a skipblock to verify and returns nil in case of success.
func (db *SkipBlockDB) VerifyLinks(sb *SkipBlock) error {
	if len(sb.BackLinkIDs) == 0 {
		return errors.New("need at least one backlink")
	}

	if err := sb.VerifyForwardSignatures(); err != nil {
		return errors.New("Wrong signatures: " + err.Error())
	}

	// Verify if we're in the responsible-list
	if !sb.ParentBlockID.IsNull() {
		parent := db.GetByID(sb.ParentBlockID)
		if parent == nil {
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

	// Verify we're referenced by our previous block
	sbBack := db.GetByID(sb.BackLinkIDs[0])
	if sbBack == nil {
		if sb.GetForwardLen() > 0 {
			log.Lvl3("Didn't find back-link, but have a good forward-link")
			return nil
		}
		return errors.New("Didn't find height-0 skipblock in db")
	}
	if err := sbBack.VerifyForwardSignatures(); err != nil {
		return err
	}
	if !sbBack.GetForward(0).Hash.Equal(sb.Hash) {
		return errors.New("didn't find our block in forward-links")
	}
	return nil
}

// GetLatest searches for the latest available block for that skipblock.
func (sbm *SkipBlockMap) GetLatest(sb *SkipBlock) (*SkipBlock, error) {
	latest := sb
	for latest.GetForwardLen() > 0 {
		latest = sbm.GetByID(latest.GetForward(latest.GetForwardLen() - 1).Hash)
		if latest == nil {
			return nil, errors.New("missing block")
		}
	}
	return latest, nil
}

// GetLatest searches for the latest available block for that skipblock.
func (db *SkipBlockDB) GetLatest(sb *SkipBlock) (*SkipBlock, error) {
	latest := sb
	// TODO this can be optimised by using multiple bucket.Get in a single transaction
	for latest.GetForwardLen() > 0 {
		latest = db.GetByID(latest.GetForward(latest.GetForwardLen() - 1).Hash)
		if latest == nil {
			return nil, errors.New("missing block")
		}
	}
	return latest, nil
}

// GetFuzzy searches for a block that resembles the given ID, if ID is not full.
// If there are multiple matching skipblocks, the first one is chosen. If none
// match, nil will be returned.
//
// The search is done in the following order:
//  1. as prefix - if none is found
//  2. as suffix - if none is found
//  3. anywhere
func (sbm *SkipBlockMap) GetFuzzy(id string) *SkipBlock {
	for _, sb := range sbm.SkipBlocks {
		if strings.HasPrefix(hex.EncodeToString(sb.Hash), id) {
			return sb
		}
	}
	for _, sb := range sbm.SkipBlocks {
		if strings.HasSuffix(hex.EncodeToString(sb.Hash), id) {
			return sb
		}
	}
	for _, sb := range sbm.SkipBlocks {
		if strings.Contains(hex.EncodeToString(sb.Hash), id) {
			return sb
		}
	}
	return nil
}

// GetFuzzy searches for a block that resembles the given ID, if ID is not full.
// If there are multiple matching skipblocks, the first one is chosen. If none
// match, nil will be returned.
//
// The search is done in the following order:
//  1. as prefix - if none is found
//  2. as suffix - if none is found
//  3. anywhere
// TODO a wrapper around db.GetByID for now
func (db *SkipBlockDB) GetFuzzy(id string) *SkipBlock {
	sbID, err := hex.DecodeString(id)
	if err != nil {
		log.Error(err.Error())
		return nil
	}
	return db.GetByID(sbID)
}

func (db *SkipBlockDB) dbStore(sb *SkipBlock) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(skipblocksBucket))
		key := sb.Hash
		val, err := network.Marshal(sb)
		if err != nil {
			return err
		}

		return b.Put(key, val)
	})
}

func (db *SkipBlockDB) dbGet(sbID SkipBlockID) (*SkipBlock, error) {
	var sb *SkipBlock
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(skipblocksBucket))

		val := b.Get(sbID)
		if val == nil {
			sb = nil
			return nil
		}

		_, sbMsg, err := network.Unmarshal(val, cothority.Suite)
		if err != nil {
			return err
		}

		sb = sbMsg.(*SkipBlock).Copy()
		return nil
	})

	return sb, err
}

func (db *SkipBlockDB) dbDump() (map[string]*SkipBlock, error) {
	chains := map[string]*SkipBlock{}
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(skipblocksBucket))
		return b.ForEach(func(k, v []byte) error {
			_, sbMsg, err := network.Unmarshal(v, cothority.Suite)
			if err != nil {
				return err
			}

			sb := sbMsg.(*SkipBlock)
			chains[string(sb.SkipChainID())] = sb
			return nil
		})
	})

	if err != nil {
		return nil, err
	}
	return chains, nil
}
