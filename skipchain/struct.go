package skipchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"go.dedis.ch/cothority/v3/blscosi/bdnproto"
	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/cothority/v3/byzcoinx"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	bbolt "go.etcd.io/bbolt"
	uuid "gopkg.in/satori/go.uuid.v1"
)

// ErrorInconsistentForwardLink is triggered when the target of a forward-link
// doesn't respect the consistency of the chain.
var ErrorInconsistentForwardLink = errors.New("found inconsistent forward-link")

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

// VerifierIDs provides helpers to compare arrays of verifierID
type VerifierIDs []VerifierID

// Equal returns true when both array contains the same verifiers
// in the same order
func (vids VerifierIDs) Equal(others []VerifierID) bool {
	if len(vids) != len(others) {
		return false
	}

	for i, vid := range vids {
		if !vid.Equal(others[i]) {
			return false
		}
	}

	return true
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
		GenesisID:     genesisID,
		Data:          data,
		Roster:        sbf.Roster,
	}
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

	// Payload is additional data that needs to be hashed by the application
	// itself into SkipBlockFix.Data. A normal use case is to set
	// SkipBlockFix.Data to the sha256 of this payload. Then the proofs
	// using the skipblocks can return simply the SkipBlockFix, as long as they
	// don't need the payload.
	Payload []byte `protobuf:"opt"`

	// SignatureScheme holds the index of the scheme to use to verify the signature.
	SignatureScheme uint32
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
	if !sb.Hash.Equal(sb.CalculateHash()) {
		// Because we extract the public keys from the block, we need to insure
		// the hash is correct with respect to what is stored
		return errors.New("Calculated hash does not match")
	}

	if sb.Roster == nil {
		return errors.New("Missing roster in the block")
	}

	publics := sb.Roster.ServicePublics(ServiceName)

	for _, fl := range sb.ForwardLink {
		if fl.IsEmpty() {
			// This means it's an empty forward-link to correctly place a higher-order
			// forward-link in place.
			continue
		}
		if err := fl.VerifyWithScheme(suite, publics, sb.SignatureScheme); err != nil {
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
		SkipBlockFix:    sb.SkipBlockFix.Copy(),
		Hash:            make([]byte, len(sb.Hash)),
		Payload:         make([]byte, len(sb.Payload)),
		ForwardLink:     make([]*ForwardLink, len(sb.ForwardLink)),
		SignatureScheme: sb.SignatureScheme,
	}
	for i, fl := range sb.ForwardLink {
		b.ForwardLink[i] = fl.Copy()
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
	if pos < 0 || pos >= sb.Height {
		return errors.New("invalid position")
	}
	if !fw.From.Equal(sb.Hash) {
		return errors.New("forward link doesn't start from this block")
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

// pathForIndex computes the highest height that can be used to go
// to the targeted index and also returns the index associated. It
// works for forward abd backward links.
func (sb *SkipBlock) pathForIndex(targetIndex int) (int, int) {
	diff := math.Log(math.Abs(float64(targetIndex - sb.Index)))
	base := math.Log(float64(sb.BaseHeight))
	// Highest forward or backward link height that can be followed.
	h := int(math.Min(diff/base, float64(sb.Height-1)))

	// We need to round the e^x operation because of the floating-point
	// precision but unlike the previous division, this will always
	// produce the index minus ε < 10^-9.
	offset := int(math.Round(math.Exp(float64(h) * base)))
	if targetIndex < sb.Index {
		// Going backwards in that case
		offset *= -1
	}

	return h, sb.Index + offset
}

// SignatureProtocol returns the name of the byzcoinx protocols that should
// be used to sign forward links coming from this block. The first protocol
// shall be used for forward link level 0 and the second shall be used to
// create higher level forward links.
func (sb *SkipBlock) SignatureProtocol() (string, string) {
	switch sb.SignatureScheme {
	case BlsSignatureSchemeIndex:
		return bftNewBlock, bftFollowBlock
	case BdnSignatureSchemeIndex:
		return bdnNewBlock, bdnFollowBlock
	default:
		return "", ""
	}
}

// CalculateHash hashes all fixed fields of the skipblock.
func (sb *SkipBlock) CalculateHash() SkipBlockID {
	hash := sha256.New()
	for _, i := range []int{sb.Index, sb.Height, sb.MaximumHeight,
		sb.BaseHeight} {
		err := binary.Write(hash, binary.LittleEndian, int32(i))
		if err != nil {
			panic("error writing to hash:" + err.Error())
		}
	}

	for _, bl := range sb.BackLinkIDs {
		hash.Write(bl)
	}
	for _, v := range sb.VerifierIDs {
		hash.Write(v[:])
	}
	hash.Write(sb.GenesisID)
	hash.Write(sb.Data)
	if sb.Roster != nil {
		for _, pub := range sb.Roster.Publics() {
			_, err := pub.MarshalTo(hash)
			if err != nil {
				panic("couldn't marshall point to hash: " + err.Error())
			}
		}
	}

	// For backwards compatibility, the signature scheme is only added to
	// the hash when different from the previous default (== 0)
	if sb.SignatureScheme > 0 {
		err := binary.Write(hash, binary.LittleEndian, sb.SignatureScheme)
		if err != nil {
			panic("error writing to hash: " + err.Error())
		}
	}

	buf := hash.Sum(nil)
	return buf
}

func (sb *SkipBlock) updateHash() SkipBlockID {
	sb.Hash = sb.CalculateHash()
	return sb.Hash
}

// Proof is a list of blocks from the genesis to the latest block
// using the shortest path
type Proof []*SkipBlock

// Search returns the block with the given index or nil
func (sbs Proof) Search(index int) *SkipBlock {
	for _, sb := range sbs {
		if sb.Index == index {
			return sb
		}
	}

	return nil
}

// Verify checks that the proof is correct by checking individual
// blocks and their back and forward links
func (sbs Proof) Verify() error {
	if len(sbs) == 0 {
		return errors.New("Empty list of blocks")
	}

	if sbs[0].Index != 0 {
		return errors.New("First element must be a genesis")
	}

	return sbs.verifyChain()
}

// VerifyFromID checks that the proof is correct starting from a given
// block and verifies the back and forward links up to the last block
func (sbs Proof) VerifyFromID(id SkipBlockID) error {
	if len(sbs) == 0 {
		return errors.New("Empty list of blocks")
	}

	// the hash will be checked afterwards
	if !sbs[0].Hash.Equal(id) {
		return errors.New("Proof does not start with the correct block")
	}

	return sbs.verifyChain()
}

func (sbs Proof) verifyChain() error {
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
			if err := fl.VerifyWithScheme(suite, sb.Roster.ServicePublics(ServiceName), sb.SignatureScheme); err != nil {
				return err
			}

			if !sbs[i+1].Hash.Equal(fl.To) || !fl.From.Equal(sb.Hash) {
				return errors.New("Wrong targets for the forward link")
			}
		}
	}

	return nil
}

// Signature schemes should be ordered by robustness such that for any
// x < y, S(x) <= S(y) where S(i) quantify the security of the signature
// scheme at index i
const (
	// BlsSignatureSchemeIndex is the index for BLS signatures
	BlsSignatureSchemeIndex = uint32(iota)
	// BdnSignatureSchemeIndex is the index for BDN signatures
	BdnSignatureSchemeIndex
)

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
	return fl.VerifyWithScheme(suite, pubs, 0)
}

// VerifyWithScheme checks the signature against a list of public keys with
// a given scheme. The list must correspond to the block roster to match the
// signature. It returns nil if the signature is correct, or an error if not.
func (fl *ForwardLink) VerifyWithScheme(suite *pairing.SuiteBn256, pubs []kyber.Point, scheme uint32) error {
	if bytes.Compare(fl.Signature.Msg, fl.Hash()) != 0 {
		return errors.New("wrong hash of forward link")
	}

	switch scheme {
	case BlsSignatureSchemeIndex:
		return protocol.BlsSignature(fl.Signature.Sig).Verify(suite, fl.Signature.Msg, pubs)
	case BdnSignatureSchemeIndex:
		return bdnproto.BdnSignature(fl.Signature.Sig).Verify(suite, fl.Signature.Msg, pubs)
	default:
		return errors.New("unknown signature scheme")
	}
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
	*bbolt.DB
	bucketName []byte
	// latestBlocks is used as a simple caching mechanism
	latestBlocks map[string]SkipBlockID
	latestMutex  sync.Mutex
	callback     func(SkipBlockID) error
}

// NewSkipBlockDB returns an initialized SkipBlockDB structure.
func NewSkipBlockDB(db *bbolt.DB, bn []byte) *SkipBlockDB {
	return &SkipBlockDB{
		DB:           db,
		bucketName:   bn,
		latestBlocks: map[string]SkipBlockID{},
	}
}

// GetStatus is a function that returns the status report of the db.
func (db *SkipBlockDB) GetStatus() *onet.Status {
	out := make(map[string]string)
	err := db.DB.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(db.bucketName))
		s := b.Stats()
		out["Blocks"] = strconv.Itoa(s.KeyN)

		total := s.BranchInuse + s.LeafInuse
		out["Bytes"] = strconv.Itoa(total)
		return nil
	})
	if err != nil {
		log.Error(err)
		return nil
	}
	return &onet.Status{Field: out}
}

// GetByID returns a new copy of the skip-block or nil if it doesn't exist
func (db *SkipBlockDB) GetByID(sbID SkipBlockID) *SkipBlock {
	var result *SkipBlock
	err := db.View(func(tx *bbolt.Tx) error {
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
	err := db.Update(func(tx *bbolt.Tx) error {
		for i, sb := range blocks {
			sbOld, err := db.getFromTx(tx, sb.Hash)
			if err != nil {
				return errors.New("failed to get skipblock with error: " + err.Error())
			}
			if sbOld != nil {
				numFL := len(sbOld.ForwardLink)
				// If this skipblock already exists, only copy forward-links and
				// new children.
				if len(sb.ForwardLink) > numFL {
					for i, fl := range sb.ForwardLink[numFL:] {
						if fl.IsEmpty() {
							// Ignore empty links.
							continue
						}

						publics := sbOld.Roster.ServicePublics(ServiceName)

						if err := fl.VerifyWithScheme(suite, publics, sb.SignatureScheme); err != nil {
							// Only keep a log of the failing forward links but keep trying others.
							log.Error("Got a known block with wrong signature in forward-link with error: " + err.Error())
							continue
						}

						target, err := db.getFromTx(tx, fl.To)
						if err != nil {
							return err
						}
						// Only check the target height if it exists because the block might
						// not yet be stored (e.g. catch up).
						if target != nil {
							diff := math.Log(float64(target.Index - sbOld.Index))
							base := math.Log(float64(sbOld.BaseHeight))
							if int(diff/base) != i+numFL {
								log.Errorf("Received a forward link with an invalid height: %x/%d", sb.Hash, i+numFL)
								continue
							}
						}

						if err := sbOld.AddForwardLink(fl, i+numFL); err != nil {
							log.Error(err)
						}
					}
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

				// As we don't store the target height in the forward-link, there is no
				// way to ensure that it will be stored at the right height if the target
				// block is not yet discovered like in a catch up scenario that will end
				// up in this path.
				// Deprecated: This is a notice for v4 to add the target height in the
				// forward-link so conodes can sign it.
				if len(sb.ForwardLink) > sb.Height {
					return fmt.Errorf("found %d forward-links for a height of %d",
						len(sb.ForwardLink), sb.Height)
				}

				publics := sb.Roster.ServicePublics(ServiceName)

				for _, fl := range sb.ForwardLink {
					if !fl.IsEmpty() {
						if !fl.From.Equal(sb.Hash) {
							return ErrorInconsistentForwardLink
						}

						if err := fl.VerifyWithScheme(suite, publics, sb.SignatureScheme); err != nil {
							return errors.New("invalid forward-link signature: " + err.Error())
						}
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
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(db.bucketName))
		i = b.Stats().KeyN
		return nil
	})
	return i
}

// GetResponsible searches for the block that is responsible for sb
// - Root_Genesis - himself
// - else - it's the previous block
func (db *SkipBlockDB) GetResponsible(sb *SkipBlock) (*SkipBlock, error) {
	if sb == nil {
		log.Panic(log.Stack())
	}
	if sb.Index == 0 {
		// Root-skipchain, no other parent
		return sb, nil
	}
	if len(sb.BackLinkIDs) == 0 {
		return nil, errors.New("invalid block: no backlink")
	}
	prev := db.GetByID(sb.BackLinkIDs[0])
	if prev == nil {
		return nil, errors.New("didn't find responsible")
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
		return errors.New("didn't find height-0 skipblock in db")
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
			// During a block creation, the level-0 link is created first and then the
			// new block could still be processing.
			return latest, nil
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
	err = db.View(func(tx *bbolt.Tx) error {
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
	return sb, err
}

// GetProof returns the shortest chain from the genesis to the latest block
// using the heighest forward-links available in the local db
func (db *SkipBlockDB) GetProof(sid SkipBlockID) (sbs []*SkipBlock, err error) {
	sbs = make([]*SkipBlock, 0)

	err = db.View(func(tx *bbolt.Tx) error {
		sb, err := db.getFromTx(tx, sid)
		if err != nil {
			return err
		}

		sbs = append(sbs, sb)

		for sb != nil && len(sb.ForwardLink) > 0 {
			// In some cases, a forward-link could be stored before the
			// actual target block is so we go as far as we can when
			// following the forward-links.
			links := sb.ForwardLink
			for k := len(links) - 1; k >= 0; k-- {
				fl := links[k]
				if fl == nil || fl.IsEmpty() {
					continue
				}

				sb, err = db.getFromTx(tx, fl.To)

				if err != nil {
					return err
				}

				if sb != nil {
					k = -1
				}
			}

			if sb == nil {
				// The very latest block could still in processing but the
				// forward-link level 0 is already stored.
				return nil
			}

			// One way to insure there is no corrupted forward-link is
			// to insure the index is monotonically increasing.
			if sb.Index <= sbs[len(sbs)-1].Index {
				return ErrorInconsistentForwardLink
			}

			sbs = append(sbs, sb)
		}

		return err
	})
	return
}

// GetProofForID returns the shortest chain known from the genesis to the given
// block using the heighest forward-links available in the local db.
func (db *SkipBlockDB) GetProofForID(bid SkipBlockID) (sbs Proof, err error) {
	err = db.View(func(tx *bbolt.Tx) error {
		target, err := db.getFromTx(tx, bid)
		if err != nil {
			return err
		}
		if target == nil {
			return errors.New("couldn't find the block")
		}

		sb, err := db.getFromTx(tx, target.SkipChainID())
		if err != nil {
			return err
		}
		if sb == nil {
			// It should never happen if the previous is found.
			return errors.New("couldn't find the genesis block")
		}

		sbs = append(sbs, sb)

		for !sb.Hash.Equal(bid) && len(sb.ForwardLink) > 0 {
			diff := math.Log(float64(target.Index - sb.Index))
			base := math.Log(float64(sb.BaseHeight))
			maxHeight := 0
			if base != 0 {
				maxHeight = int(math.Min(diff/base, float64(len(sb.ForwardLink)-1)))
			}

			id := sb.ForwardLink[maxHeight].To
			sb, err = db.getFromTx(tx, id)
			if err != nil {
				return err
			}

			if sb == nil {
				return errors.New("couldn't find one of the blocks")
			}

			if sb.Index <= sbs[len(sbs)-1].Index {
				return ErrorInconsistentForwardLink
			}

			sbs = append(sbs, sb)
		}

		return nil
	})

	return
}

// GetSkipchains returns all latest skipblocks from all skipchains.
func (db *SkipBlockDB) GetSkipchains() (map[string]*SkipBlock, error) {
	return db.getAllSkipchains()
}

// RemoveSkipchain removes all block from a given skipchain from the database.
// If the skipchain is only partial, it can skip missing blocks, as long as the
// forwardlinks are present.
func (db *SkipBlockDB) RemoveSkipchain(scid SkipBlockID) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(db.bucketName))
		sb, err := db.getFromTx(tx, scid)
		if err != nil {
			return err
		}
		for {
			err := b.Delete(sb.Hash)
			if err != nil {
				return err
			}
			if len(sb.ForwardLink) == 0 {
				return nil
			}

			var next *SkipBlock
			for _, fl := range sb.ForwardLink {
				n, err := db.getFromTx(tx, fl.To)
				if err == nil {
					next = n
					break
				}
			}
			if next == nil {
				log.Error("didn't find next block")
				return nil
			}
			sb = next
		}
	})
}

// RemoveBlock removes the given block from the database.
func (db *SkipBlockDB) RemoveBlock(blockID SkipBlockID) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(db.bucketName))
		return b.Delete(blockID)
	})
}

// storeToTx stores the skipblock into the database.
// An error is returned on failure.
// The caller must ensure that this function is called from within a valid transaction.
func (db *SkipBlockDB) storeToTx(tx *bbolt.Tx, sb *SkipBlock) error {
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
func (db *SkipBlockDB) getFromTx(tx *bbolt.Tx, sbID SkipBlockID) (*SkipBlock, error) {
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
	err := db.View(func(tx *bbolt.Tx) error {
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
	err := db.View(func(tx *bbolt.Tx) error {
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

// has returns true when the skipchain has a block in the buffer
func (sbb *skipBlockBuffer) has(sid SkipBlockID) bool {
	sbb.Lock()
	defer sbb.Unlock()

	_, ok := sbb.buffer[string(sid)]
	return ok
}

// clear deletes the current block of the given skipchain
func (sbb *skipBlockBuffer) clear(sid SkipBlockID) {
	sbb.Lock()
	defer sbb.Unlock()

	delete(sbb.buffer, string(sid))
}
