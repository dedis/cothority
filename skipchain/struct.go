package skipchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority/byzcoinx"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/satori/go.uuid.v1"
)

// How long to wait before a timeout is generated in the propagation. It is not
// set to a constant because we'd like to change it in the test.
var defaultPropagateTimeout = 15 * time.Second

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

// PolicyNewChain defines how new chains from a followed chain are treated.
type PolicyNewChain int

const (
	// NewChainNone doesn't allow any new chains from any node from this skipchain.
	NewChainNone = PolicyNewChain(iota)
	// NewChainStrictNodes allows new chains only if all nodes of the new chain
	// are present in the followed chain.
	NewChainStrictNodes
	// NewChainAnyNode allows new chains if any node from the new chain (excluded
	// ourselves) is present in this chain.
	NewChainAnyNode
)

// FollowType defines how a followed skipchain is stored
type FollowType int

const (
	// FollowID will store this skipchain-id and only allow evolution of
	// this skipchain. PolicyNewChain is supposed to be NewChainNone.
	FollowID = FollowType(iota)
	// FollowSearch asks all stored skipchains if it knows that skipchain. All
	// PolicyNewChain are allowed.
	FollowSearch
	// FollowLookup takes a ip:port where the skipchain can be found. All
	// PolicyNewChain are allowed.
	FollowLookup
)

// FollowChainType describes if nodes of a followed chain are allowed to add new
// skipchains.
type FollowChainType struct {
	Block    *SkipBlock
	NewChain PolicyNewChain
	closing  chan bool
}

type cp interface {
	CreateProtocol(string, *onet.Tree) (onet.ProtocolInstance, error)
}

// GetLatest searches for the latest version of the block by querying a
// remote node for an update.
func (fct *FollowChainType) GetLatest(us *network.ServerIdentity, p cp) error {
	log.Lvlf3("%s: fetching latest block of index %d: %x", us, fct.Block.Index, fct.Block.SkipChainID())
	t := onet.NewRoster([]*network.ServerIdentity{us, fct.Block.Roster.List[0]}).GenerateBinaryTree()
	pi, err := p.CreateProtocol(ProtocolGetBlocks, t)
	if err != nil {
		return err
	}
	pisc := pi.(*GetBlocks)
	pisc.GetBlocks = &ProtoGetBlocks{Count: 1, SBID: fct.Block.Hash}
	if err := pi.Start(); err != nil {
		return err
	}
	select {
	case sbNew := <-pisc.GetBlocksReply:
		if len(sbNew) >= 1 {
			log.Lvlf3("%s: found new block with index %d", us, sbNew[0].Index)
			fct.Block = sbNew[0]
		}
	case <-time.After(time.Second):
		return errors.New("timeout while fetching latest block")
	case <-fct.closing:
		return errors.New("closing down")
	}
	return nil
}

// AcceptNew loops through all followed chains and verifies if the new skipblock
// sb is acceptable, taking into account our identity 'us'.
func (fct *FollowChainType) AcceptNew(sb *SkipBlock, us *network.ServerIdentity) bool {
	// Fetch latest block of this skipchain
	switch fct.NewChain {
	case NewChainNone:
		return false
	case NewChainAnyNode:
		// Accept if any node of the new roster is in this roster, but exclude
		// ourselves (else it would always be true).
		for _, si1 := range sb.Roster.List {
			if us == nil || !si1.Equal(us) {
				for _, si2 := range fct.Block.Roster.List {
					if si1.Equal(si2) {
						return true
					}
				}
			}
		}
	case NewChainStrictNodes:
		for _, si1 := range sb.Roster.List {
			found := false
			if us != nil && si1.Equal(us) {
				continue
			}
			for _, si2 := range fct.Block.Roster.List {
				if si1.Equal(si2) {
					found = true
					break
				}
			}
			if !found {
				log.Lvlf2("%s: Not all nodes are in followed skipchains: NewBlock[%s] - Following[%s]",
					us, sb.Roster.List, fct.Block.Roster.List)
				return false
			}
		}
		return true
	default:
		log.Error("unknown policy")
	}
	return false
}

// Shutdown imitates the protocol-shutdown to make sure we can stop it.
func (fct *FollowChainType) Shutdown() {
	close(fct.closing)
}

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
	// that there is no new block if a newer parent is present.
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
	// GenesisID is the ID of the genesis-block. For the genesis-block, this
	// is null. The SkipBlockID() method returns the correct ID both for
	// the genesis block and for later blocks.
	GenesisID SkipBlockID
	// Data is any data to be stored in that SkipBlock
	Data []byte
	// Roster holds the roster-definition of that SkipBlock
	Roster *onet.Roster
}

// Copy returns a deep copy of SkipBlockFix
func (sbf *SkipBlockFix) Copy() *SkipBlockFix {
	backLinkIDs := make([]SkipBlockID, len(sbf.BackLinkIDs))
	for i := range backLinkIDs {
		backLinkIDs[i] = make(SkipBlockID, len(sbf.BackLinkIDs[i]))
		copy(backLinkIDs[i], sbf.BackLinkIDs[i])
	}

	verifierIDs := make([]VerifierID, len(sbf.VerifierIDs))
	copy(verifierIDs, sbf.VerifierIDs)

	parentBlockID := make(SkipBlockID, len(sbf.ParentBlockID))
	copy(parentBlockID, sbf.ParentBlockID)

	genesisID := make(SkipBlockID, len(sbf.GenesisID))
	copy(genesisID, sbf.GenesisID)

	data := make([]byte, len(sbf.Data))
	copy(data, sbf.Data)

	return &SkipBlockFix{
		Index:         sbf.Index,
		Height:        sbf.Height,
		MaximumHeight: sbf.MaximumHeight,
		BaseHeight:    sbf.BaseHeight,
		BackLinkIDs:   backLinkIDs,
		VerifierIDs:   verifierIDs,
		ParentBlockID: parentBlockID,
		GenesisID:     genesisID,
		Data:          data,
		Roster:        sbf.Roster,
	}
}

// CalculateHash hashes all fixed fields of the skipblock.
func (sbf *SkipBlockFix) CalculateHash() SkipBlockID {
	hash := sha256.New()
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
	// Hash is our Block-hash of the SkipBlockFix part.
	Hash SkipBlockID

	// ForwardLink will be calculated once future SkipBlocks are
	// available
	ForwardLink []*ForwardLink
	// SkipLists that depend on us, given as the first SkipBlock - can
	// be a Data or a Roster SkipBlock
	ChildSL []SkipBlockID

	// Payload is additional data that needs to be hashed by the application
	// itself into SkipBlockFix.Data. A normal use case is to set
	// SkipBlockFix.Data to the sha256 of this payload. Then the proofs
	// using the skipblocks can return simply the SkipBlockFix, as long as they
	// don't need the payload.
	Payload []byte `protobuf:"opt"`
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
	publics := sb.Roster.ServicePublics(ServiceName)

	for _, fl := range sb.ForwardLink {
		if fl.IsEmpty() {
			// This means it's an empty forward-link to correctly place a higher-order
			// forward-link in place.
			continue
		}
		if err := fl.Verify(suite, publics); err != nil {
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
	b := &SkipBlock{
		SkipBlockFix: sb.SkipBlockFix.Copy(),
		Hash:         make([]byte, len(sb.Hash)),
		Payload:      make([]byte, len(sb.Payload)),
		ForwardLink:  make([]*ForwardLink, len(sb.ForwardLink)),
		ChildSL:      make([]SkipBlockID, len(sb.ChildSL)),
	}
	for i, fl := range sb.ForwardLink {
		b.ForwardLink[i] = fl.Copy()
	}
	for i, child := range sb.ChildSL {
		b.ChildSL[i] = make(SkipBlockID, len(child))
		copy(b.ChildSL[i], child)
	}
	copy(b.Hash, sb.Hash)
	copy(b.Payload, sb.Payload)
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

// AddForward stores the forward-link.
// DEPRECATION NOTICE: this method will disappear in onet.v3
func (sb *SkipBlock) AddForward(fw *ForwardLink) {
	log.Warn("this is deprecated, because it might create 'holes'")
	sb.ForwardLink = append(sb.ForwardLink, fw)
}

// AddForwardLink stores the forward-link at the indicated position. If the
// forwardlink at pos already exists, it returns an error.
func (sb *SkipBlock) AddForwardLink(fw *ForwardLink, pos int) error {
	if len(sb.ForwardLink) > pos || pos < 0 {
		return errors.New("this forward-link already exists or invalid position")
	}
	for len(sb.ForwardLink) <= pos {
		sb.ForwardLink = append(sb.ForwardLink, &ForwardLink{})
	}
	sb.ForwardLink[pos] = fw
	return nil
}

// GetForward returns copy of the forward-link at position i. It returns nil if no link
// at that level exists.
func (sb *SkipBlock) GetForward(i int) *ForwardLink {
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

// Proof is a list of blocks from the genesis to the latest block
// using the shortest path
type Proof []*SkipBlock

// Verify checks that the proof is correct by checking individual
// blocks and their back and forward links
func (sbs Proof) Verify() error {
	if len(sbs) == 0 {
		return errors.New("Empty list of blocks")
	}

	if sbs[0].Index != 0 {
		return errors.New("First element must be a genesis")
	}

	for i, sb := range sbs {
		if !sb.CalculateHash().Equal(sb.Hash) {
			return errors.New("Wrong hash")
		}
		if i > 0 {
			// Check if there is a back link to the previous block
			hit := false
			for _, bl := range sb.BackLinkIDs {
				if bl.Equal(sbs[i-1].Hash) {
					hit = true
				}
			}

			if !hit {
				return errors.New("Missing backlink")
			}
		}
		if i < len(sbs)-1 {
			// Check if there is a forward link to the next block
			if len(sb.ForwardLink) == 0 {
				return errors.New("Missing forward links")
			}

			fl := sb.ForwardLink[len(sb.ForwardLink)-1]
			if err := fl.Verify(pairing.NewSuiteBn256(), sb.Roster.ServicePublics(ServiceName)); err != nil {
				return err
			}

			if !sbs[i+1].Hash.Equal(fl.To) || !fl.From.Equal(sb.Hash) {
				return errors.New("Wrong targets for the forward link")
			}
		}
	}

	return nil
}

// ForwardLink can be used to jump from old blocks to newer
// blocks. Depending on the BaseHeight and MaximumHeight, older
// rosters are asked to sign direct links to new blocks.
type ForwardLink struct {
	// From - where this forward link comes from
	From SkipBlockID
	// To - where this forward link points to
	To SkipBlockID
	// NewRoster is only set to non-nil if the From block has a
	// different roster from the To-block.
	NewRoster *onet.Roster
	// Signature is calculated on the
	// sha256(From.Hash()|To.Hash()|NewRoster)
	// In the case that NewRoster is nil, the signature is
	// calculated on the sha256(From.Hash()|To.Hash())
	Signature byzcoinx.FinalSignature
}

// NewForwardLink creates a new forwardlink structure with
// the From, To, and NewRoster initialized. If the roster in
// From and To is identitcal, NewRoster will be nil.
func NewForwardLink(from, to *SkipBlock) *ForwardLink {
	fl := &ForwardLink{
		From: from.Hash,
		To:   to.Hash,
	}

	if from.Roster != nil && to.Roster != nil &&
		!from.Roster.ID.Equal(to.Roster.ID) {
		fl.NewRoster = to.Roster
	}
	return fl
}

// Hash is calculated as
// sha256(From.Hash()|To.Hash()|NewRoster.ID), except
// if NewRoster is nil, then it is calculated as
// sha256(From.Hash()|To.Hash())
func (fl *ForwardLink) Hash() SkipBlockID {
	hash := sha256.New()
	hash.Write(fl.From)
	hash.Write(fl.To)
	if fl.NewRoster != nil {
		hash.Write(fl.NewRoster.ID[:])
	}
	return hash.Sum(nil)
}

// Copy makes a deep copy of a ForwardLink
func (fl *ForwardLink) Copy() *ForwardLink {
	var newRoster *onet.Roster
	if fl.NewRoster != nil {
		newRoster = onet.NewRoster(fl.NewRoster.List)
		newRoster.ID = onet.RosterID([uuid.Size]byte(fl.NewRoster.ID))
	}
	return &ForwardLink{
		Signature: byzcoinx.FinalSignature{
			Sig: append([]byte{}, fl.Signature.Sig...),
			Msg: append([]byte{}, fl.Signature.Msg...),
		},
		From:      append([]byte{}, fl.From...),
		To:        append([]byte{}, fl.To...),
		NewRoster: newRoster,
	}
}

// Verify checks the signature against a list of public keys. The list must
// correspond to the block roster to match the signature.
// It returns nil if the signature is correct, or an error if not.
func (fl *ForwardLink) Verify(suite *pairing.SuiteBn256, pubs []kyber.Point) error {
	if bytes.Compare(fl.Signature.Msg, fl.Hash()) != 0 {
		return errors.New("wrong hash of forward link")
	}

	// This calculation must match the one in byzcoinx.
	return fl.Signature.Sig.Verify(suite, fl.Signature.Msg, pubs)
}

// IsEmpty indicates whether this forwardlink is merely a placeholder for
// higher-order forwardlinks to be in the correct place.
func (fl *ForwardLink) IsEmpty() bool {
	return fl.From.IsNull() || fl.To.IsNull()
}

// SkipBlockDB holds the database to the skipblocks.
// This is used for verification, so that all links can be followed.
// It is a wrapper to embed bolt.DB.
type SkipBlockDB struct {
	*bolt.DB
	bucketName []byte
	// latestBlocks is used as a simple caching mechanism
	latestBlocks map[string]SkipBlockID
	latestMutex  sync.Mutex
	callback     func(SkipBlockID) error
}

// NewSkipBlockDB returns an initialized SkipBlockDB structure.
func NewSkipBlockDB(db *bolt.DB, bn []byte) *SkipBlockDB {
	return &SkipBlockDB{
		DB:           db,
		bucketName:   bn,
		latestBlocks: map[string]SkipBlockID{},
	}
}

// GetStatus is a function that returns the status report of the db.
func (db *SkipBlockDB) GetStatus() *onet.Status {
	out := make(map[string]string)
	db.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(db.bucketName))
		s := b.Stats()
		out["Blocks"] = strconv.Itoa(s.KeyN)

		total := s.BranchInuse + s.LeafInuse
		out["Bytes"] = strconv.Itoa(total)
		return nil
	})
	return &onet.Status{Field: out}
}

// GetByID returns a new copy of the skip-block or nil if it doesn't exist
func (db *SkipBlockDB) GetByID(sbID SkipBlockID) *SkipBlock {
	var result *SkipBlock
	err := db.View(func(tx *bolt.Tx) error {
		sb, err := db.getFromTx(tx, sbID)
		if err != nil {
			return err
		}
		result = sb
		return nil
	})

	if err != nil {
		log.Error(err)
	}
	return result
}

// StoreBlocks stores the set of blocks in the boltdb in a transaction,
// so that the db is consistent at every moment.
func (db *SkipBlockDB) StoreBlocks(blocks []*SkipBlock) ([]SkipBlockID, error) {
	var result []SkipBlockID
	err := db.Update(func(tx *bolt.Tx) error {
		for i, sb := range blocks {
			sbOld, err := db.getFromTx(tx, sb.Hash)
			if err != nil {
				return errors.New("failed to get skipblock with error: " + err.Error())
			}
			if sbOld != nil {
				// If this skipblock already exists, only copy forward-links and
				// new children.
				if len(sb.ForwardLink) > len(sbOld.ForwardLink) {
					for i, fl := range sb.ForwardLink {
						if i < len(sbOld.ForwardLink) || fl.IsEmpty() {
							// Don't overwrite existing forwardlinks and ignore empty links
							continue
						}

						publics := sbOld.Roster.ServicePublics(ServiceName)

						if err := fl.Verify(suite, publics); err != nil {
							return errors.New("Got a known block with wrong signature in forward-link with error: " + err.Error())
						}
						if err := sbOld.AddForwardLink(fl, i); err != nil {
							log.Error(err)
							return nil
						}
					}
				}
				if len(sb.ChildSL) > len(sbOld.ChildSL) {
					sbOld.ChildSL = append(sbOld.ChildSL, sb.ChildSL[len(sbOld.ChildSL):]...)
				}
				err := db.storeToTx(tx, sbOld)
				if err != nil {
					return err
				}
			} else {
				if !db.HasForwardLink(sb) {
					found := false
					for j := 0; j < i; j++ {
						for _, fl := range blocks[j].ForwardLink {
							if fl.To.Equal(sb.Hash) {
								found = true
							}
						}
					}
					if !found {
						return fmt.Errorf("Tried to store unlinkable block: %+v", sb.SkipBlockFix)
					}
				}
				err := db.storeToTx(tx, sb)
				if err != nil {
					return err
				}
				db.latestUpdate(sb)
			}
			result = append(result, sb.Hash)
		}
		return nil
	})

	// Run the callback if it exists, we have to do this outside of the
	// boltdb transaction because the callback might also make updates to
	// the database. Otherwise there will be a deadlock.
	if db.callback != nil {
		for _, r := range result {
			if err := db.callback(r); err != nil {
				log.Errorf("Error while adding block %x: %s", r, err)
			}
		}
	}

	return result, err
}

// Store stores the given SkipBlock in the service-list
func (db *SkipBlockDB) Store(sb *SkipBlock) SkipBlockID {
	ids, err := db.StoreBlocks([]*SkipBlock{sb})
	if err != nil {
		log.Error(err)
	}
	if len(ids) > 0 {
		return ids[0]
	}
	return nil
}

// HasForwardLink verififes if sb can be accepted in the database by searching
// for a forwardlink of any level.
func (db *SkipBlockDB) HasForwardLink(sb *SkipBlock) bool {
	if sb.Index == 0 {
		// Genesis blocks never have a reference to them.
		return true
	}

	// Any non-genesis blocks need to be referenced by a previous block.
	for i, bl := range sb.BackLinkIDs {
		prev := db.GetByID(bl)
		if prev != nil {
			if len(prev.ForwardLink) > i {
				if prev.ForwardLink[i].To.Equal(sb.Hash) {
					return true
				}
			}
		}
	}
	return false
}

func (db *SkipBlockDB) latestUpdate(sb *SkipBlock) {
	db.latestMutex.Lock()
	defer db.latestMutex.Unlock()
	idStr := string(sb.SkipChainID())
	latest, exists := db.latestBlocks[idStr]
	if !exists {
		db.latestBlocks[idStr] = sb.Hash
	} else {
		old := db.GetByID(latest)
		if old == nil || old.Index < sb.Index {
			log.Lvlf3("updating sc %x: storing index %d", idStr, sb.Index)
			db.latestBlocks[idStr] = sb.Hash
		}
	}
}

// Length returns the actual length using mutexes
func (db *SkipBlockDB) Length() int {
	var i int
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(db.bucketName))
		i = b.Stats().KeyN
		return nil
	})
	return i
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
	if !sbBack.GetForward(0).Hash().Equal(sb.Hash) {
		return errors.New("didn't find our block in forward-links")
	}
	return nil
}

// GetLatestByID returns the latest skipblock of a skipchain
// given its ID.
func (db *SkipBlockDB) GetLatestByID(genID SkipBlockID) (*SkipBlock, error) {
	gen := db.GetByID(genID)
	if gen == nil {
		return nil, fmt.Errorf("cannot find genesis block %x", genID)
	}
	return db.GetLatest(gen)
}

// GetLatest searches for the latest available block for that skipblock.
func (db *SkipBlockDB) GetLatest(sb *SkipBlock) (*SkipBlock, error) {
	if sb == nil {
		return nil, errors.New("got nil skipblock")
	}
	latest := sb
	db.latestMutex.Lock()
	latestID, exists := db.latestBlocks[string(sb.SkipChainID())]
	if exists {
		latest = db.GetByID(latestID)
		if latest == nil {
			latest = sb
		}
	}
	db.latestMutex.Unlock()

	// TODO this can be optimised by using multiple bucket.Get in a single transaction
	for latest.GetForwardLen() > 0 {
		next := db.GetByID(latest.GetForward(latest.GetForwardLen() - 1).To)
		if next == nil {
			return latest, errors.New("missing block")
		}
		latest = next
	}
	db.latestUpdate(latest)
	return latest, nil
}

// GetFuzzy searches for a block that resembles the given ID.
// If there are multiple matching skipblocks, the first one is chosen. If none
// match, nil will be returned.
//
// The search is done in the following order:
//  1. as prefix - if none is found
//  2. as suffix - if none is found
//  3. anywhere
func (db *SkipBlockDB) GetFuzzy(id string) (*SkipBlock, error) {
	match, err := hex.DecodeString(id)
	if err != nil {
		return nil, errors.New("Failed to decode " + id)
	}
	if len(match) == 0 {
		return nil, errors.New("id is empty")
	}

	var sb *SkipBlock
	db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(db.bucketName)).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, match) {
				_, msg, err := network.Unmarshal(v, suite)
				if err != nil {
					return errors.New("Unmarshal failed with error: " + err.Error())
				}
				sb = msg.(*SkipBlock).Copy()
				return nil
			}
		}
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasSuffix(k, match) {
				_, msg, err := network.Unmarshal(v, suite)
				if err != nil {
					return errors.New("Unmarshal failed with error: " + err.Error())
				}
				sb = msg.(*SkipBlock).Copy()
				return nil
			}
		}
		return nil
	})
	return sb, nil
}

// GetProof returns the shortest chain from the genesis to the latest block
// using the heighest forward-links available in the local db
func (db *SkipBlockDB) GetProof(sid SkipBlockID) (sbs []*SkipBlock, err error) {
	sbs = make([]*SkipBlock, 0)

	err = db.View(func(tx *bolt.Tx) error {
		sb, err := db.getFromTx(tx, sid)
		if err != nil {
			return err
		}

		sbs = append(sbs, sb)

		for len(sb.ForwardLink) > 0 {
			id := sb.ForwardLink[len(sb.ForwardLink)-1].To
			sb, err = db.getFromTx(tx, id)

			if err != nil {
				return err
			}

			if sb == nil {
				return errors.New("Couldn't find one of the blocks")
			}

			sbs = append(sbs, sb)
		}

		return err
	})
	return
}

// GetSkipchains returns all latest skipblocks from all skipchains.
func (db *SkipBlockDB) GetSkipchains() (map[string]*SkipBlock, error) {
	return db.getAll()
}

// storeToTx stores the skipblock into the database.
// An error is returned on failure.
// The caller must ensure that this function is called from within a valid transaction.
func (db *SkipBlockDB) storeToTx(tx *bolt.Tx, sb *SkipBlock) error {
	key := sb.Hash
	val, err := network.Marshal(sb)
	if err != nil {
		return err
	}
	return tx.Bucket([]byte(db.bucketName)).Put(key, val)
}

// getFromTx returns the skipblock identified by sbID.
// nil is returned if the key does not exist.
// An error is thrown if marshalling fails.
// The caller must ensure that this function is called from within a valid transaction.
func (db *SkipBlockDB) getFromTx(tx *bolt.Tx, sbID SkipBlockID) (*SkipBlock, error) {
	val := tx.Bucket([]byte(db.bucketName)).Get(sbID)
	if val == nil {
		return nil, nil
	}

	// For some reason boltdb changes the val before Unmarshal finishes. When
	// copying the value into a buffer, there is no SIGSEGV anymore.
	buf := make([]byte, len(val))
	copy(buf, val)
	_, sbMsg, err := network.Unmarshal(buf, suite)
	if err != nil {
		return nil, err
	}

	return sbMsg.(*SkipBlock).Copy(), nil
}

// getAll returns all the data in the database as a map
// This function performs a single transaction,
// the caller should not perform operations that may requires a view of the
// database that is consistent at the time of the function call.
func (db *SkipBlockDB) getAll() (map[string]*SkipBlock, error) {
	data := map[string]*SkipBlock{}
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(db.bucketName))
		return b.ForEach(func(k, v []byte) error {
			_, sbMsg, err := network.Unmarshal(v, suite)
			if err != nil {
				return err
			}
			sb, ok := sbMsg.(*SkipBlock)
			if ok {
				data[string(sb.Hash)] = sb
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}
	return data, nil
}

// getAllSkipchains returns each of the skipchains in the database
// in the form of a map from skipblock ID to the latest block.
func (db *SkipBlockDB) getAllSkipchains() (map[string]*SkipBlock, error) {
	gen := make(map[string]*SkipBlock)

	// Loop over all blocks. If we see a new genesis block we
	// have not seen, remember it. If we see a higher Index than what
	// we have, replace it.
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(db.bucketName))
		return b.ForEach(func(k, v []byte) error {
			_, sbMsg, err := network.Unmarshal(v, suite)
			if err != nil {
				return err
			}
			sb, ok := sbMsg.(*SkipBlock)
			if ok {
				k := string(sb.SkipChainID())
				if cur, ok := gen[k]; ok {
					if cur.Index < sb.Index {
						gen[k] = sb
					}
				} else {
					gen[k] = sb
				}
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return gen, nil
}

// skipBlockBuffer will cache a proposed block when the conode has
// verified it and it will later store it in the DB after the protocol
// has succeeded.
// Blocks are stored per skipchain to prevent the cache to grow and it
// is cleared after a new block is added (again per skipchain)
type skipBlockBuffer struct {
	buffer map[string]*SkipBlock
	sync.Mutex
}

func newSkipBlockBuffer() *skipBlockBuffer {
	return &skipBlockBuffer{
		buffer: make(map[string]*SkipBlock),
	}
}

// add stores the block using the skipchain ID as a key
func (sbb *skipBlockBuffer) add(block *SkipBlock) {
	sbb.Lock()
	defer sbb.Unlock()

	sbb.buffer[string(block.SkipChainID())] = block
}

// get returns the block if the skipchain ID hits with the
// correct block ID
func (sbb *skipBlockBuffer) get(sid SkipBlockID, id SkipBlockID) *SkipBlock {
	sbb.Lock()
	defer sbb.Unlock()

	block, ok := sbb.buffer[string(sid)]
	if !ok || !block.Hash.Equal(id) {
		return nil
	}

	return block
}

// clear deletes the current block of the given skipchain
func (sbb *skipBlockBuffer) clear(sid SkipBlockID) {
	sbb.Lock()
	defer sbb.Unlock()

	delete(sbb.buffer, string(sid))
}
