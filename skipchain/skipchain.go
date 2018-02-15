// Package skipchain implements a service in the cothority that
// keeps track of a skipchain. It offers API-calls to create
// new skipchains, add blocks to existing skipchains, and
// request updates to known skipchain.
//
// The basic strcture needed from a clients point of view is
// Client, that has all the methods defined on it to interact
// with a skipchain.
package skipchain

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/bftcosi"
	"github.com/dedis/cothority/cosi/crypto"
	"github.com/dedis/cothority/messaging"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/satori/go.uuid.v1"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "Skipchain"
const bftNewBlock = "SkipchainBFTNew"
const bftFollowBlock = "SkipchainBFTFollow"
const storageKey = "skipchainconfig"

func init() {
	onet.RegisterNewService(ServiceName, newSkipchainService)
	network.RegisterMessages(&Storage{})
}

// Service handles adding new SkipBlocks
type Service struct {
	*onet.ServiceProcessor
	db             *SkipBlockDB
	propagate      messaging.PropagationFunc
	verifiers      map[VerifierID]SkipBlockVerifier
	newBlocksMutex sync.Mutex
	newBlocks      map[string]bool
	storageMutex   sync.Mutex
	Storage        *Storage
	bftTimeout     time.Duration
	propTimeout    time.Duration
}

// Storage is saved to disk.
type Storage struct {
	// Follow is a slice of latest blocks that point to skipchains that are allowed
	// to create new blocks
	Follow []FollowChainType
	// FollowIDs is a slice of IDs that are allowed to ask us to sign and store
	// new blocks for their skipchain.
	FollowIDs []SkipBlockID
	// Clients is a list of public keys of clients that have successfully linked
	// to this service. Once a client is linked to a service, only blocks signed
	// by this client will be allowed.
	Clients []kyber.Point
}

// ErrorProcessing happens when two clients are trying to add to the
// same skipchain at the same time.
var ErrorProcessing = errors.New("this skipchain-id is currently processing a block")

// StoreSkipBlock stores a new skipblock in the system. This can be either a
// genesis-skipblock, that will create a new skipchain, or a new skipblock,
// that will be added to an existing chain.
//
// The conode servicing the request needs to be part of the actual valid latest
// skipblock, else it will fail.
//
// It takes a hash for the latest valid SkipBlock and a SkipBlock
// that will be verified. If the verification returns true, the new SkipBlock
// will be stored, else it will be discarded and an error will be returned.
//
// If the latest block given is nil it will create a new skipchain and store
// the given block as genesis-block.
//
// If the latest block is non-nil and exists, the skipblock will be added to the
// skipchain after verification that it fits and no other block already has been
// added.
func (s *Service) StoreSkipBlock(psbd *StoreSkipBlock) (*StoreSkipBlockReply, error) {
	// Initial checks on the proposed block.
	prop := psbd.NewBlock
	if !s.ServerIdentity().Equal(prop.Roster.Get(0)) {
		return nil, errors.New(
			"only leader is allowed to add blocks")
	}
	if len(s.Storage.Clients) > 0 {
		if psbd.Signature == nil {
			return nil, errors.New(
				"cannot create new skipblock without authentication")
		}
		if !s.authenticate(psbd.NewBlock.CalculateHash(), *psbd.Signature) {
			return nil, errors.New(
				"wrong signature for this skipchain")
		}
	}
	var prev *SkipBlock
	var changed []*SkipBlock

	if psbd.LatestID.IsNull() {
		// A new chain is created
		log.Lvl3("Creating new skipchain with roster", psbd.NewBlock.Roster.List)
		prop.Index = 0
		prop.Height = prop.MaximumHeight
		prop.ForwardLink = make([]*ForwardLink, 0)
		// genesis block has a random back-link, so that two
		// identical genesis blocks have a different ID.
		var bl [32]byte
		random.Bytes(bl[:], random.New())
		prop.BackLinkIDs = []SkipBlockID{SkipBlockID(bl[:])}
		prop.GenesisID = nil
		prop.updateHash()
		err := s.verifyBlock(prop)
		if err != nil {
			return nil, err
		}
		if !s.newBlockStart(prop) {
			return nil, ErrorProcessing
		}
		defer s.newBlockEnd(prop)

		if !prop.ParentBlockID.IsNull() {
			parent := s.db.GetByID(prop.ParentBlockID)
			if parent == nil {
				return nil, errors.New(
					"Didn't find parent")
			}
			parent.ChildSL = append(parent.ChildSL, prop.Hash)
			changed = append(changed, parent)
		}
		changed = append(changed, prop)

	} else {

		// We're appending a block to an existing chain
		log.Lvlf3("Adding block with roster %+v to %x", psbd.NewBlock.Roster.List, psbd.LatestID)
		prev = s.db.GetByID(psbd.LatestID)
		if prev == nil {
			// Did not find the block they claim is the latest. So
			// if this is a chain we are supposed to be following
			// (is friendly), then talk to peers to catch up.

			if !s.blockIsFriendly(psbd.NewBlock) {
				log.Lvlf2("%s: block is not friendly: %x", s.ServerIdentity(), psbd.NewBlock.Hash)
				return nil, errors.New("chain not followed")
			}

			gen := s.db.GetByID(psbd.NewBlock.SkipChainID())
			if gen == nil {
				return nil, errors.New("unknown latest block, unknown chain-id")
			}

			// If we know of this chain, try to sync it.
			latest := s.findLatest(gen)
			log.Lvlf2("Catching up chain %x from index %v", gen.Hash, latest.Index)
			err := s.syncChain(latest.Roster, latest.Hash)
			if err != nil {
				return nil, errors.New("failed to catch up")

			}
			prev = s.db.GetByID(psbd.LatestID)
			if prev == nil {
				return nil, errors.New(
					"Didn't find latest block, even after catchup")

			}
		}
		if i, _ := prev.Roster.Search(s.ServerIdentity().ID); i < 0 {
			return nil, errors.New(
				"We're not responsible for latest block")

		}
		if len(prev.ForwardLink) > 0 {
			return nil, errors.New(
				"the latest block already has a follower")

		}
		if !s.newBlockStart(prev) {
			return nil, errors.New(
				"this skipchain-id is currently processing a block")

		}
		defer s.newBlockEnd(prev)

		prop.MaximumHeight = prev.MaximumHeight
		prop.BaseHeight = prev.BaseHeight
		prop.ParentBlockID = nil
		prop.VerifierIDs = prev.VerifierIDs
		prop.Index = prev.Index + 1
		prop.GenesisID = prev.SkipChainID()
		index := prop.Index
		for prop.Height = 1; index%prop.BaseHeight == 0; prop.Height++ {
			index /= prop.BaseHeight
			if prop.Height >= prop.MaximumHeight {
				break
			}
		}
		log.Lvl4("Found height", prop.Height, "for index", prop.Index,
			"and maxHeight", prop.MaximumHeight, "and base", prop.BaseHeight)
		prop.BackLinkIDs = make([]SkipBlockID, prop.Height)
		pointer := prev
		for h := range prop.BackLinkIDs {
			for pointer.Height < h+1 {
				pointer = s.db.GetByID(pointer.BackLinkIDs[0])
				if pointer == nil {
					return nil, errors.New(
						"Didn't find convenient SkipBlock for height " +
							strconv.Itoa(h))

				}
			}
			prop.BackLinkIDs[h] = pointer.Hash
		}
		prop.updateHash()

		// Only check changing roster, or if this is the block after the genesis-block,
		// as we don't verify the roster for the genesis-block.
		log.Lvl3("Checking if all nodes from roster accept block")
		if !prev.Roster.ID.Equal(prop.Roster.ID) || prop.Index == 1 {
			if !s.willNodesAcceptBlock(prop) {
				return nil, errors.New(
					"node refused to accept new roster")

			}
		}

		if err := s.addForwardLink(prev, prop); err != nil {
			return nil, errors.New(
				"Couldn't get forward signature on block: " + err.Error())
		}
		changed = append(changed, prev, prop)
		log.Lvl3("Asking forward-links from all linked blocks")
		for i, bl := range prop.BackLinkIDs[1:] {
			back := s.db.GetByID(bl)
			if back == nil {
				return nil, errors.New(
					"Didn't get skipblock in back-link")

			}
			if err := s.forwardSignature(
				&ForwardSignature{
					TargetHeight: i + 1,
					Previous:     back.Hash,
					Newest:       prop,
				}); err != nil {
				// This is not a critical failure - we have at least
				// one forward-link
				log.Lvl1("Couldn't get old block to sign: " + err.Error())
			}
		}
	}
	log.Lvlf3("Propagate %d blocks", len(changed))
	if err := s.startPropagation(changed); err != nil {
		return nil, errors.New(
			"Couldn't propagate new blocks: " + err.Error())

	}
	reply := &StoreSkipBlockReply{
		Previous: prev,
		Latest:   prop,
	}
	log.Lvlf3("Block added, replying. New latest is: %x", prop.Hash)
	return reply, nil
}

// GetUpdateChain returns a slice of SkipBlocks which describe the part of the
// skipchain from the latest block the caller knows to the latest
// SkipBlock we know. The last block in the returned slice of blocks is
// not guaranteed to have no forward links. It is up to the caller
// to continue following forward links with the new roster if necessary.
func (s *Service) GetUpdateChain(guc *GetUpdateChain) (network.Message, error) {
	block := s.db.GetByID(guc.LatestID)
	if block == nil {
		return nil, errors.New("Couldn't find latest skipblock")
	}

	blocks := []*SkipBlock{block.Copy()}
	log.Lvlf3("Starting to search chain at %s", s.Context.ServerIdentity())
	for block.GetForwardLen() > 0 {
		link := block.ForwardLink[block.GetForwardLen()-1]
		next := s.db.GetByID(link.To)
		if next == nil {
			// Next not found means that maybe the roster
			// has evolved and we are no longer aware of
			// this chain. The caller will be responsible
			// to issue a new GetUpdateChain with the
			// latest Roster to keep traversing.
			break
		} else {
			if i, _ := next.Roster.Search(s.ServerIdentity().ID); i < 0 {
				// Likewise for the case where we know the block,
				// but we are no longer in the Roster, stop searching.
				break
			}
		}
		block = next
		blocks = append(blocks, next.Copy())
	}
	log.Lvl3("Found", len(blocks), "blocks")
	reply := &GetUpdateChainReply{Update: blocks}

	return reply, nil
}

// Search the local DB starting at bl and finding the latest block we know.
func (s *Service) findLatest(bl *SkipBlock) *SkipBlock {
	for {
		if len(bl.ForwardLink) == 0 {
			return bl
		}
		next := bl.ForwardLink[len(bl.ForwardLink)-1].To
		nextBl := s.db.GetByID(next)
		if nextBl == nil {
			return bl
		}
		bl = nextBl
	}
}

// syncChain communicates with conodes in the Roster via getBlocks
// in order traverse the chain and save the blocks locally.
func (s *Service) syncChain(roster *onet.Roster, latest SkipBlockID) error {
	// loop on getBlocks, fetching 10 at a time
	for {
		blocks, err := s.getBlocks(roster, latest, 10)
		if err != nil {
			return err
		}

		for _, sb := range blocks {
			if err := sb.VerifyForwardSignatures(); err != nil {
				return err
			}
			s.db.Store(sb)
			if len(sb.ForwardLink) == 0 {
				return nil
			}
			latest = sb.Hash
		}
	}
}

// getBlocks uses ProtocolGetBlocks to return up to n blocks, traversing the
// skiplist forward from id. It contacts a random subgroup of some of the nodes
// in the roster, in order to find an answer, even in the case that a few
// nodes in the network are down.
func (s *Service) getBlocks(roster *onet.Roster, id SkipBlockID, n int) ([]*SkipBlock, error) {
	subCount := (len(roster.List)-1)/3 + 1
	r := roster.RandomSubset(s.ServerIdentity(), subCount)
	tr := r.GenerateStar()
	pi, err := s.CreateProtocol(ProtocolGetBlocks, tr)
	if err != nil {
		return nil, err
	}

	pisc := pi.(*GetBlocks)
	pisc.GetBlocks = &ProtoGetBlocks{
		SBID:     id,
		Count:    n,
		Skipping: true,
	}
	if err := pi.Start(); err != nil {
		log.ErrFatal(err)
	}
	select {
	case result := <-pisc.GetBlocksReply:
		return result, nil
	case <-time.After(s.propTimeout):
		return nil, errors.New("timeout waiting for GetBlocks reply")
	}
}

// getLastBlock talks one of the servers in roster in order to find the latest
// block that it knows about.
func (s *Service) getLastBlock(roster *onet.Roster, latest SkipBlockID) (*SkipBlock, error) {
	// loop on getBlocks, fetching 10 at a time
	for {
		blocks, err := s.getBlocks(roster, latest, 10)
		if err != nil {
			return nil, err
		}
		if len(blocks) == 0 {
			return nil, errors.New("getLastBlock got unexpected empty list")
		}
		// last block of this batch
		lb := blocks[len(blocks)-1]

		if len(lb.ForwardLink) == 0 {
			return lb, nil
		}
		latest = lb.Hash
	}
}

// GetSingleBlock searches for the given block and returns it. If no such block is
// found, a nil is returned.
func (s *Service) GetSingleBlock(id *GetSingleBlock) (*SkipBlock, error) {
	sb := s.db.GetByID(id.ID)
	if sb == nil {
		return nil, errors.New(
			"No such block")

	}
	return sb, nil
}

// GetSingleBlockByIndex searches for the given block and returns it. If no such block is
// found, a nil is returned.
func (s *Service) GetSingleBlockByIndex(id *GetSingleBlockByIndex) (*SkipBlock, error) {
	sb := s.db.GetByID(id.Genesis)
	if sb == nil {
		return nil, errors.New(
			"No such genesis-block")

	}
	if sb.Index == id.Index {
		return sb, nil
	}
	for len(sb.ForwardLink) > 0 {
		sb = s.db.GetByID(sb.ForwardLink[0].To)
		if sb == nil {
			return nil, errors.New("didn't find block in forward link")
		}
		if sb.Index == id.Index {
			return sb, nil
		}
	}
	return nil, errors.New(
		"No block with this index found")

}

// GetAllSkipchains returns a list of all known skipchains
func (s *Service) GetAllSkipchains(id *GetAllSkipchains) (*GetAllSkipchainsReply, error) {
	// Write all known skipblocks to a map, thus removing double blocks.
	chains, err := s.db.getAll()
	if err != nil {
		return nil, err
	}

	reply := &GetAllSkipchainsReply{
		SkipChains: make([]*SkipBlock, 0, len(chains)),
	}
	for _, sb := range chains {
		reply.SkipChains = append(reply.SkipChains, sb)
	}
	return reply, nil
}

// CreateLinkPrivate checks if the given public key is signed with our private
// key and stores it in the list of allowed clients if it is true.
func (s *Service) CreateLinkPrivate(link *CreateLinkPrivate) (*EmptyReply, error) {
	msg, err := link.Public.MarshalBinary()
	if err != nil {
		return nil, errors.New("couldn't marshal public key: " + err.Error())
	}
	if err = schnorr.Verify(cothority.Suite, s.ServerIdentity().Public, msg, link.Signature); err != nil {
		return nil, errors.New("wrong signature on public key: " + err.Error())
	}
	s.storageMutex.Lock()
	s.Storage.Clients = append(s.Storage.Clients, link.Public)
	s.storageMutex.Unlock()
	s.save()
	return &EmptyReply{}, nil
}

// Unlink removes a public key from the list of linked nodes.
// Authentication to unlink is done by a signature on the
// following message:
// "unlink:" + byte representation of the public key to be
// removed
func (s *Service) Unlink(unlink *Unlink) (*EmptyReply, error) {
	msg, err := unlink.Public.MarshalBinary()
	if err != nil {
		return &EmptyReply{}, err
	}
	msg = append([]byte("unlink:"), msg...)
	found := false
	for _, pub := range s.Storage.Clients {
		if pub.Equal(unlink.Public) {
			found = true
			break
		}
	}
	if !found {
		return &EmptyReply{}, errors.New("didn't find public key in clients")
	}
	err = schnorr.Verify(s.Suite(), unlink.Public, msg, unlink.Signature)
	if err != nil {
		return &EmptyReply{}, err
	}
	client := -1
	for i, pub := range s.Storage.Clients {
		if pub.Equal(unlink.Public) {
			client = i
			break
		}
	}
	if client == -1 {
		return &EmptyReply{}, errors.New("didn't find this clients public key")
	}
	s.Storage.Clients = append(s.Storage.Clients[:client], s.Storage.Clients[client+1:]...)
	s.save()
	return &EmptyReply{}, nil
}

// Listlink returns a list of all public keys that are linked
// with this conode and are allowed to do administrative
// tasks.
func (s *Service) Listlink(list *Listlink) (*ListlinkReply, error) {
	reply := &ListlinkReply{}
	for _, pub := range s.Storage.Clients {
		reply.Publics = append(reply.Publics, pub)
	}
	return reply, nil
}

// AddFollow adds a new skipchain to be followed
func (s *Service) AddFollow(add *AddFollow) (*EmptyReply, error) {
	msg := []byte{byte(add.Follow)}
	msg = append(add.SkipchainID, msg...)
	msg = append(msg, []byte(add.Conode)...)
	if !s.verifySigs(msg, add.Signature) {
		return &EmptyReply{}, errors.New("wrong signature of unknown signer")
	}

	s.storageMutex.Lock()
	defer s.save()
	defer s.storageMutex.Unlock()
	switch add.Follow {
	case FollowID:
		log.Lvlf2("%s FollowChain %x", s.ServerIdentity(), add.SkipchainID)
		s.Storage.FollowIDs = append(s.Storage.FollowIDs, add.SkipchainID)
	case FollowSearch:
		// First search if anybody knows that SkipBlockID
		sis := map[string]*network.ServerIdentity{}
		for _, fct := range s.Storage.Follow {
			for _, si := range fct.Block.Roster.List {
				sis[si.ID.String()] = si
			}
		}
		// TODO: this is really not good and will fail if we have too many blocks.
		sbs, err := s.db.getAll()
		if err != nil {
			return nil, errors.New("couldn't load db of all blocks")
		}
		for sc := range sbs {
			for _, si := range s.db.GetByID(SkipBlockID(sc)).Roster.List {
				sis[si.ID.String()] = si
			}
		}

		found := false
		for _, si := range sis {
			roster := onet.NewRoster([]*network.ServerIdentity{si})
			last, err := s.getLastBlock(roster, add.SkipchainID)
			if err != nil {
				log.Lvl1(s.ServerIdentity(), "could not get last block: ", err)
			} else {
				if last.SkipChainID().Equal(add.SkipchainID) {
					s.Storage.Follow = append(s.Storage.Follow,
						FollowChainType{
							Block:    last,
							NewChain: add.NewChain,
						})
					found = true
					break
				}
			}
		}
		if !found {
			return nil, errors.New("didn't find that skipchain-id")
		}
		log.Lvlf2("%s FollowSearch %s %x", s.ServerIdentity(), add.Conode, add.SkipchainID)
	case FollowLookup:
		si := network.NewServerIdentity(cothority.Suite.Point().Null(), network.NewAddress(network.PlainTCP, add.Conode))
		roster := onet.NewRoster([]*network.ServerIdentity{si})
		last, err := s.getLastBlock(roster, add.SkipchainID)
		if err != nil {
			return nil, errors.New("didn't find skipchain at given address")
		}
		if !last.SkipChainID().Equal(add.SkipchainID) {
			return nil, errors.New("returned block is not correct")
		}
		s.Storage.Follow = append(s.Storage.Follow,
			FollowChainType{
				Block:    last,
				NewChain: add.NewChain,
			})
		log.Lvlf2("%s FollowLookup %x", s.ServerIdentity(), add.SkipchainID)
	default:
		return nil, errors.New("unknown follow type")
	}
	return &EmptyReply{}, nil
}

// DelFollow searches for that skipchain in the follower
// list and deletes it if it is there.
func (s *Service) DelFollow(del *DelFollow) (*EmptyReply, error) {
	msg := append([]byte("delfollow:"), del.SkipchainID...)
	if !s.verifySigs(msg, del.Signature) {
		return &EmptyReply{}, errors.New("wrong signature of unknown signer")
	}
	deleted := false
	for i, scid := range s.Storage.FollowIDs {
		if scid.Equal(del.SkipchainID) {
			s.Storage.FollowIDs = append(s.Storage.FollowIDs[:i],
				s.Storage.FollowIDs[i+1:]...)
			deleted = true
			break
		}
	}
	for i, fct := range s.Storage.Follow {
		if fct.Block.SkipChainID().Equal(del.SkipchainID) {
			s.Storage.Follow = append(s.Storage.Follow[:i],
				s.Storage.Follow[i+1:]...)
			deleted = true
			break
		}
	}
	if !deleted {
		return &EmptyReply{}, errors.New("didn't find any block of that id")
	}
	s.save()
	return &EmptyReply{}, nil
}

// ListFollow returns the skipchain-ids that are followed
func (s *Service) ListFollow(list *ListFollow) (*ListFollowReply, error) {
	reply := &ListFollowReply{}
	msg, err := s.ServerIdentity().Public.MarshalBinary()
	if err != nil {
		return reply, errors.New("couldn't marshal public key")
	}
	msg = append([]byte("listfollow:"), msg...)
	if !s.verifySigs(msg, list.Signature) {
		return reply, errors.New("wrong signature of unknown signer")
	}
	if len(s.Storage.Follow) > 0 {
		reply.Follow = &s.Storage.Follow
	}
	if len(s.Storage.FollowIDs) > 0 {
		reply.FollowIDs = &s.Storage.FollowIDs
	}
	return reply, nil
}

// IsPropagating returns true if there is at least one propagation running.
func (s *Service) IsPropagating() bool {
	s.newBlocksMutex.Lock()
	defer s.newBlocksMutex.Unlock()
	return len(s.newBlocks) > 0
}

// GetDB returns a pointer to the internal database.
func (s *Service) GetDB() *SkipBlockDB {
	return s.db
}

// NewProtocol intercepts the creation of the skipblock protocol and
// initialises the necessary variables.
func (s *Service) NewProtocol(ti *onet.TreeNodeInstance, conf *onet.GenericConfig) (pi onet.ProtocolInstance, err error) {
	if ti.ProtocolName() == ProtocolExtendRoster {
		// Start by getting latest blocks of all followers
		pi, err = NewProtocolExtendRoster(ti)
		if err == nil {
			pier := pi.(*ExtendRoster)
			pier.Followers = &s.Storage.Follow
			pier.FollowerIDs = s.Storage.FollowIDs
			pier.DB = s.db
			pier.SaveCallback = s.save
		}
	}
	if ti.ProtocolName() == ProtocolGetBlocks {
		pi, err = NewProtocolGetBlocks(ti)
		if err == nil {
			pigu := pi.(*GetBlocks)
			pigu.DB = s.db
		}
	}
	return
}

// AddClientKey can be used by other services to add a key so
// they can store new Blocks
func (s *Service) AddClientKey(pub kyber.Point) {
	for _, p := range s.Storage.Clients {
		if p.Equal(pub) {
			return
		}
	}
	s.Storage.Clients = append(s.Storage.Clients, pub)
	s.save()
}

func (s *Service) verifySigs(msg, sig []byte) bool {
	// If there are no clients, all signatures verify.
	if len(s.Storage.Clients) == 0 {
		return true
	}

	for _, cl := range s.Storage.Clients {
		if schnorr.Verify(cothority.Suite, cl, msg, sig) == nil {
			return true
		}
	}
	return false
}

// addForwardLink verifies if the new block is valid. If it is not valid, it
// returns with an error.
// If it finds a valid block, a forward-link will be added and a BFT-signature
// requested.
func (s *Service) addForwardLink(src, dst *SkipBlock) error {
	if src.GetForwardLen() > 0 {
		return errors.New("already have forward-link at this height")
	}

	// create the message we want to sign for this round
	roster := src.Roster
	log.Lvlf3("%s is adding forward-link to %s: %d->%d", s.ServerIdentity(),
		roster.List, src.Index, dst.Index)
	fs := &ForwardSignature{
		TargetHeight: len(src.ForwardLink),
		Previous:     src.Hash,
		Newest:       dst,
	}
	data, err := network.Marshal(fs)
	if err != nil {
		return fmt.Errorf("Couldn't marshal block: %s", err.Error())
	}
	fwd := NewForwardLink(src, dst)
	sig, err := s.startBFT(bftNewBlock, roster, fwd.Hash(), data)
	if err != nil {
		log.Warn("startBFT failed with", err)
		return err
	}
	fwd.Signature = *sig

	fwl := s.db.GetByID(src.Hash).ForwardLink
	log.Lvlf3("%s adds forward-link to %s: %d->%d - fwlinks:%v", s.ServerIdentity(),
		roster.List, src.Index, dst.Index, fwl)
	if len(fwl) > 0 {
		return errors.New("forward-link got signed during our signing")
	}
	src.ForwardLink = []*ForwardLink{fwd}
	if err = src.VerifyForwardSignatures(); err != nil {
		return errors.New("Wrong BFT-signature: " + err.Error())
	}
	s.startPropagation([]*SkipBlock{src})
	return nil
}

// verifyNewBlock makes sure that a signature-request for a forward-link
// is valid.
func (s *Service) bftVerifyNewBlock(msg []byte, data []byte) (out bool) {
	log.Lvlf4("%s verifying block %x", s.ServerIdentity(), msg)
	_, fsInt, err := network.Unmarshal(data, cothority.Suite)
	if err != nil {
		log.Error("Couldn't unmarshal ForwardSignature", data)
		return false
	}
	fs, ok := fsInt.(*ForwardSignature)
	if !ok {
		log.Errorf("got unexpected type %T", fsInt)
		return false
	}
	prevSB := s.db.GetByID(fs.Previous)
	if prevSB == nil {
		if !s.blockIsFriendly(fs.Newest) {
			log.Lvlf2("%s: block is not friendly: %x", s.ServerIdentity(), fs.Newest.Hash)
			return
		}
		log.Lvl3(s.ServerIdentity(), "Didn't find src-skipblock, trying to sync")
		if err := s.syncChain(fs.Newest.Roster, fs.Previous); err != nil {
			log.Error("failed to sync skipchain", err)
			return false
		}
		prevSB = s.db.GetByID(fs.Previous)
		if prevSB == nil {
			log.Error(s.ServerIdentity(), "Didn't find src-skipblock")
			return false
		}
	}

	fl := NewForwardLink(prevSB, fs.Newest)
	if bytes.Compare(fl.Hash(), msg) != 0 {
		log.Lvlf2("Hash of ForwardLink is different from msg %x %x", msg, fl.Hash())
		return false
	}

	if !fs.Newest.BackLinkIDs[0].Equal(fs.Previous) {
		log.Lvl2("Backlink does not point to previous block:", prevSB.Index, fs.Newest.Index)
		return false
	}
	if len(prevSB.ForwardLink) > 0 {
		log.Lvl2("previous block already has forward-link")
		return false
	}

	ok = func() bool {
		for _, ver := range fs.Newest.VerifierIDs {
			f, ok := s.verifiers[ver]
			if !ok {
				log.Lvlf2("Found no user verification for %x", ver)
				return false
			}
			if !f(fl.To, fs.Newest) {
				log.Lvlf2("verification function failed: %v %s", f, ver)
				return false
			}
		}
		return true
	}()
	return ok
}

// forwardSignature receives a signature request of a newly accepted block.
// It only needs the 2nd-newest block and the forward-link.
func (s *Service) forwardSignature(fs *ForwardSignature) error {
	if fs.TargetHeight >= len(fs.Newest.BackLinkIDs) {
		return errors.New("This backlink-height doesn't exist")
	}
	target := s.db.GetByID(fs.Newest.BackLinkIDs[fs.TargetHeight])
	if target == nil {
		return errors.New("Didn't find target-block")
	}
	if !fs.Previous.Equal(target.Hash) {
		return errors.New("TargetHeight backlink doesn't correspond to previous")
	}
	data, err := network.Marshal(fs)
	if err != nil {
		return err
	}
	fl := NewForwardLink(target, fs.Newest)
	sig, err := s.startBFT(bftFollowBlock, target.Roster, fl.Hash(), data)
	if err != nil {
		return errors.New("Couldn't get signature: " + err.Error())
	}
	log.Lvl1("Adding forward-link to", target.Index)

	fl.Signature = *sig
	if !target.Roster.ID.Equal(fs.Newest.Roster.ID) {
		fl.NewRoster = fs.Newest.Roster
	}
	target.AddForward(fl)
	s.startPropagation([]*SkipBlock{target})
	return nil
}

// verifyFollowBlock makes sure that a signature-request for a forward-link
// is valid.
func (s *Service) bftVerifyFollowBlock(msg []byte, data []byte) bool {
	err := func() error {
		_, fsInt, err := network.Unmarshal(data, cothority.Suite)
		if err != nil {
			return err
		}
		fs, ok := fsInt.(*ForwardSignature)
		if !ok {
			return errors.New("Didn't receive a ForwardSignature")
		}
		newest := fs.Newest
		if len(newest.BackLinkIDs) <= fs.TargetHeight {
			return errors.New("Asked for signing too high a backlink")
		}
		previous := s.db.GetByID(newest.BackLinkIDs[0])
		if previous == nil {
			return errors.New("Didn't find newest block")
		}
		if len(previous.ForwardLink) == 0 {
			return errors.New("Previous doesn't have forward link - cannot verify it's valid")
		}
		highest := previous.ForwardLink[len(previous.ForwardLink)-1]
		if err := highest.Verify(cothority.Suite, previous.Roster.Publics()); err != nil {
			return errors.New("Wrong forward-link signature: " + err.Error())
		}
		if !highest.To.Equal(newest.Hash) {
			return errors.New("Previous doesn't point to newest")
		}
		target := s.db.GetByID(newest.BackLinkIDs[fs.TargetHeight])
		if target == nil {
			return errors.New("Don't have target-block")
		}
		if target.GetForwardLen() >= fs.TargetHeight+1 {
			return errors.New("Already have forward-link at height " +
				strconv.Itoa(fs.TargetHeight+1))
		}
		if !target.SkipChainID().Equal(newest.SkipChainID()) {
			return errors.New("Target and newest not from same skipchain")
		}
		fl := NewForwardLink(target, fs.Newest)
		if bytes.Compare(fl.Hash(), msg) != 0 {
			return errors.New("Hash to sign doesn't correspond to ForwardSignature")
		}
		return nil
	}()
	if err != nil {
		log.Error(err)
		return false
	}
	return true
}

// startBFT starts a BFT-protocol with the given parameters.
func (s *Service) startBFT(proto string, roster *onet.Roster, msg, data []byte) (*bftcosi.BFTSignature, error) {
	bf := 2
	if len(roster.List)-1 > 2 {
		bf = len(roster.List) - 1
	}
	tree := roster.GenerateNaryTreeWithRoot(bf, s.ServerIdentity())
	if tree == nil {
		return nil, errors.New("couldn't form tree")
	}
	node, err := s.CreateProtocol(proto, tree)
	if err != nil {
		return nil, fmt.Errorf("couldn't create new node: %s", err.Error())
	}
	root := node.(*bftcosi.ProtocolBFTCoSi)

	switch len(roster.List) {
	case 0:
		return nil, errors.New("found empty Roster")
	case 1:
		pubs := []kyber.Point{s.ServerIdentity().Public}
		co := crypto.NewCosi(cothority.Suite, root.Private(), pubs)
		co.CreateCommitment(cothority.Suite.RandomStream())
		co.CreateChallenge(msg)
		co.CreateResponse()
		// This is when using kyber-cosi
		// r, c := cosi.Commit(Suite, random.Stream)
		// ch, err := cosi.Challenge(Suite, c, s.ServerIdentity().Public, msg)
		// if err != nil {
		// 	return nil, errors.New("couldn't create cosi-signature: " + err.Error())
		// }
		// resp, err := cosi.Response(Suite, root.Private(), r, ch)
		// if err != nil {
		// 	return nil, errors.New("couldn't create cosi-signature: " + err.Error())
		// }
		// coSig, err := cosi.Sign(Suite, c, resp, nil)
		// if err != nil {
		// 	return nil, errors.New("couldn't create cosi-signature: " + err.Error())
		// }
		sig := &bftcosi.BFTSignature{
			Msg:        msg,
			Sig:        co.Signature(),
			Exceptions: []bftcosi.Exception{},
		}
		if crypto.VerifySignature(cothority.Suite, pubs, msg, sig.Sig) != nil {
			return nil, errors.New("failed in cosi")
		}
		return sig, nil
	}

	// Start the protocol

	// Register the function generating the protocol instance
	root.Msg = msg
	root.Data = data

	if s.bftTimeout != 0 {
		root.Timeout = s.bftTimeout
	}

	// function that will be called when protocol is finished by the root
	done := make(chan bool)
	root.RegisterOnDone(func() {
		done <- true
	})
	go node.Start()
	var sig *bftcosi.BFTSignature
	select {
	case <-done:
		sig = root.Signature()
		if sig.Sig == nil {
			return nil, errors.New("couldn't sign forward-link")
		}
		return sig, nil
	case <-time.After(s.propTimeout):
		return nil, errors.New("timed out while waiting for signature")
	}
}

// PropagateSkipBlock will save a new SkipBlock
func (s *Service) propagateSkipBlock(msg network.Message) {
	sbs, ok := msg.(*PropagateSkipBlocks)
	if !ok {
		log.Error("Couldn't convert to slice of SkipBlocks")
		return
	}
	for _, sb := range sbs.SkipBlocks {
		if err := sb.VerifyForwardSignatures(); err != nil {
			log.Error(err)
			return
		}
		if !s.blockIsFriendly(sb) {
			log.Lvlf2("%s: block is not friendly: %x", s.ServerIdentity(), sb.Hash)
			return
		}
		s.db.Store(sb)
	}
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func (s *Service) registerVerification(v VerifierID, f SkipBlockVerifier) error {
	s.verifiers[v] = f
	return nil
}

// checkBlock makes sure the basic parameters of a block are correct and returns
// an error if something fails.
func (s *Service) verifyBlock(sb *SkipBlock) error {
	if sb.MaximumHeight <= 0 {
		return errors.New("Set a maximumHeight > 0")
	}
	if sb.BaseHeight <= 0 {
		return errors.New("Set a baseHeight > 0")
	}
	if sb.MaximumHeight > sb.BaseHeight {
		return errors.New("maximumHeight must be smaller or equal baseHeight")
	}
	if sb.Index < 0 {
		return errors.New("Can't have an index < 0")
	}
	if len(sb.BackLinkIDs) <= 0 {
		return errors.New("Need at least one backlinkID")
	}
	if sb.Height < 1 {
		return errors.New("Minimum height is 1")
	}
	if sb.Height > sb.MaximumHeight {
		return errors.New("Height must be <= maximumHeight")
	}
	if sb.Roster == nil {
		return errors.New("Need a roster")
	}
	return nil
}

// notify other services about new/updated skipblock
func (s *Service) startPropagation(blocks []*SkipBlock) error {
	log.Lvl3("Starting to propagate for service", s.ServerIdentity())
	siMap := map[string]*network.ServerIdentity{}
	// Add all rosters of all blocks - everybody needs to be contacted
	for _, block := range blocks {
		for _, si := range block.Roster.List {
			siMap[uuid.UUID(si.ID).String()] = si
		}
	}
	siList := make([]*network.ServerIdentity, 0, len(siMap))
	for _, si := range siMap {
		siList = append(siList, si)
	}
	roster := onet.NewRoster(siList)

	replies, err := s.propagate(roster, &PropagateSkipBlocks{blocks}, s.propTimeout)
	if err != nil {
		return err
	}
	if replies != len(roster.List) {
		log.Lvl1(s.ServerIdentity(), "Only got", replies, "out of", len(roster.List))
	}
	return nil
}

func (s *Service) newBlockStart(sb *SkipBlock) bool {
	s.newBlocksMutex.Lock()
	defer s.newBlocksMutex.Unlock()
	if _, processing := s.newBlocks[string(sb.Hash)]; processing {
		return false
	}
	if len(s.newBlocks) > 0 {
		return false
	}
	s.newBlocks[string(sb.Hash)] = true
	return true
}

func (s *Service) newBlockEnd(sb *SkipBlock) bool {
	s.newBlocksMutex.Lock()
	defer s.newBlocksMutex.Unlock()
	if _, processing := s.newBlocks[string(sb.Hash)]; !processing {
		return false
	}
	delete(s.newBlocks, string(sb.Hash))
	return true
}

// authenticate searches if this node or any follower-node can verify the
// schnorr-signature.
func (s *Service) authenticate(msg []byte, sig []byte) bool {
	if err := schnorr.Verify(cothority.Suite, s.ServerIdentity().Public, msg, sig); err == nil {
		return true
	}
	s.storageMutex.Lock()
	defer s.storageMutex.Unlock()
	for _, fct := range s.Storage.Follow {
		for _, si := range fct.Block.Roster.List {
			if err := schnorr.Verify(cothority.Suite, si.Public, msg, sig); err == nil {
				return true
			}
		}
	}
	for _, cl := range s.Storage.Clients {
		if err := schnorr.Verify(cothority.Suite, cl, msg, sig); err == nil {
			return true
		}
	}
	return false
}

// blockIsFriendly searches if all members of the new block are followed
// by this node.
func (s *Service) blockIsFriendly(sb *SkipBlock) bool {
	s.storageMutex.Lock()
	defer s.storageMutex.Unlock()

	// If no skipchains are stored, allow everything
	if len(s.Storage.Follow) == 0 {
		return true
	}
	// accept all blocks that are already stored with us.
	if s.db.GetByID(sb.SkipChainID()) != nil {
		return true
	}
	// Also accept blocks that are stored in the FollowIDs
	for _, id := range s.Storage.FollowIDs {
		if id.Equal(sb.SkipChainID()) {
			return true
		}
	}

	// Accept if we're the root.
	index, _ := sb.Roster.Search(s.ServerIdentity().ID)
	if index == 0 {
		return true
	}

	// For each Follow, find out if it permits this chain.
	for _, fct := range s.Storage.Follow {
		err := fct.GetLatest(s.ServerIdentity(), s)
		if err != nil {
			log.Error(err)
		}
		if fct.AcceptNew(sb, s.ServerIdentity()) {
			return true
		}
	}
	return false
}

// willNodesAcceptBlock returns true if all nodes in the block accept it.
func (s *Service) willNodesAcceptBlock(block *SkipBlock) bool {
	pi, err := s.CreateProtocol(ProtocolExtendRoster, block.Roster.GenerateNaryTree(len(block.Roster.List)))
	if err != nil {
		return false
	}
	pisc := pi.(*ExtendRoster)
	pisc.ExtendRoster = &ProtoExtendRoster{Block: *block}
	pisc.Start()
	sigs := <-pisc.ExtendRosterReply
	// TODO: store the sigs in the skipblock to prove the other node was OK
	// the final -1 is to exclude the root
	return len(sigs) >= len(block.Roster.List)-(len(block.Roster.List)-1)/3-1
}

// Saves s.Storage into the DB. The blocks themselves are stored as they
// are added.
func (s *Service) save() {
	s.storageMutex.Lock()
	defer s.storageMutex.Unlock()
	log.Lvl3("Saving service")
	err := s.Save(storageKey, s.Storage)
	if err != nil {
		log.Error("Couldn't save file:", err)
	}
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	msg, err := s.Load(storageKey)
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	var ok bool
	s.Storage, ok = msg.(*Storage)
	if !ok {
		return errors.New("data of wrong type")
	}
	return nil
}

func newSkipchainService(c *onet.Context) (onet.Service, error) {
	db, bucket := c.GetAdditionalBucket("skipblocks")
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		db:               NewSkipBlockDB(db, bucket),
		Storage:          &Storage{},
		verifiers:        map[VerifierID]SkipBlockVerifier{},
		newBlocks:        make(map[string]bool),
		propTimeout:      defaultPropagateTimeout,
	}

	if err := s.tryLoad(); err != nil {
		return nil, err
	}
	log.ErrFatal(s.RegisterHandlers(s.StoreSkipBlock, s.GetUpdateChain,
		s.GetSingleBlock, s.GetSingleBlockByIndex, s.GetAllSkipchains,
		s.CreateLinkPrivate, s.Unlink, s.AddFollow, s.ListFollow,
		s.DelFollow, s.Listlink))
	s.ServiceProcessor.RegisterStatusReporter("Skipblock", s.db)

	if err := s.registerVerification(VerifyBase, s.verifyFuncBase); err != nil {
		return nil, err
	}
	if err := s.registerVerification(VerifyRoot, s.verifyFuncRoot); err != nil {
		return nil, err
	}
	if err := s.registerVerification(VerifyControl, s.verifyFuncControl); err != nil {
		return nil, err
	}
	if err := s.registerVerification(VerifyData, s.verifyFuncData); err != nil {
		return nil, err
	}

	var err error
	s.propagate, err = messaging.NewPropagationFunc(c, "SkipchainPropagate", s.propagateSkipBlock, -1)
	if err != nil {
		return nil, err
	}
	s.ProtocolRegister(bftNewBlock, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, s.bftVerifyNewBlock)
	})
	s.ProtocolRegister(bftFollowBlock, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, s.bftVerifyFollowBlock)
	})
	return s, nil
}
