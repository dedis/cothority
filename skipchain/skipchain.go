// Package skipchain implements a service in the cothority that keeps track of
// a skipchain.
//
// It offers API-calls to create new skipchains, add blocks to existing
// skipchains, and request updates to known skipchain.
//
// The basic strcture needed from a clients point of view is Client, that has
// all the methods defined on it to interact with a skipchain.
//
// Please consult the README for more information
// https://github.com/dedis/cothority/blob/master/skipchain/README.md.
package skipchain

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"time"

	"golang.org/x/xerrors"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/cothority/v3/byzcoinx"
	"go.dedis.ch/cothority/v3/messaging"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "Skipchain"
const bftNewBlock = "SkipchainBFTNew"
const bftFollowBlock = "SkipchainBFTFollow"
const bdnNewBlock = "SkipchainBDNNew"
const bdnFollowBlock = "SkipchainBDNFollow"

var storageKey = []byte("skipchainconfig")
var dbVersion = 1
var suite = pairing.NewSuiteBn256()

var sid onet.ServiceID

func init() {
	sid, _ = onet.RegisterNewServiceWithSuite(ServiceName, suite, newSkipchainService)
	network.RegisterMessages(&Storage{})
}

// Service handles adding new SkipBlocks
type Service struct {
	*onet.ServiceProcessor
	db                      *SkipBlockDB
	blockBuffer             *skipBlockBuffer
	propagateGenesis        messaging.PropagationFunc
	propagateForwardLink    messaging.PropagationFunc
	propagateProof          messaging.PropagationFunc
	verifiers               map[VerifierID]SkipBlockVerifier
	storageMutex            sync.Mutex
	Storage                 *Storage
	bftTimeout              time.Duration
	propTimeout             time.Duration
	chains                  chainLocker
	verifyNewBlockBuffer    sync.Map
	verifyFollowBlockBuffer sync.Map
	closed                  bool
	closedMutex             sync.Mutex
	working                 sync.WaitGroup
	closing                 chan bool

	// disableForwardLink is useful in testing mode
	disableForwardLink bool
}

type chainLocker struct {
	sync.Mutex
	// the key type is string because []byte is not allowed
	// in Go maps as keys.
	chains map[string]*sync.Mutex
	// a count of how many locks are currently held
	locks int
}

var errTimeout = errors.New("timeout waiting to lock chain")

func (cl *chainLocker) lock(chain SkipBlockID) {
	cl.Lock()
	// Lazy initializtion.
	if cl.chains == nil {
		cl.chains = make(map[string]*sync.Mutex)
	}

	cl.locks++
	if l, ok := cl.chains[string(chain)]; ok {
		cl.Unlock()
		l.Lock()
		return
	}

	l := new(sync.Mutex)
	l.Lock()
	cl.chains[string(chain)] = l
	cl.Unlock()
	return
}

func (cl *chainLocker) unlock(chain SkipBlockID) {
	key := string(chain)
	cl.Lock()
	l := cl.chains[key]
	if l != nil {
		l.Unlock()
		cl.locks--
	}
	// It is not possible to delete the entry from the map in a non-racy way.
	// Consider 3 goroutines. #1 has the lock and is here unlocking. #2 is
	// sleeping on the lock. #3 has just received a block for this chain, but has
	// not tried to lock it. When #1 unlocks, #2 wakes up and runs, locking the
	// same mutex that #1 just unlocked. If #1 deletes the mutex from the map,
	// later #2 will unlock it and could just ignore the fact that it is no longer
	// in the map. But if #3 tries to lock while #2 has it locked, but after #1 has
	// removed it from the map, #3 makes a new mutex, and gets the lock. Now #2 and #3
	// are both holding different locks that they each think is the lock for chainId.
	// Not ok.

	cl.Unlock()
}

func (cl *chainLocker) numLocks() (ret int) {
	cl.Lock()
	ret = cl.locks
	cl.Unlock()
	return
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

// StoreSkipBlock stores a new skipblock in the system. This can be either a
// genesis-skipblock, that will create a new skipchain, or a new skipblock,
// that will be added to an existing chain.
//
// The conode servicing the request needs to be part of the actual valid latest
// skipblock, else it will fail.
//
// It takes TargetSkipChainID, which is the chain that the client wants to
// append to, and a new SkipBlock.  The new SkipBlock will be verified. If it
// passes, then the block is appended to the chain, otherwise an error is
// returned.
//
// If TargetSkipChainID is an empty slice, the service will create a new
// skipchain and store the given block as genesis-block.
func (s *Service) StoreSkipBlock(psbd *StoreSkipBlock) (*StoreSkipBlockReply, error) {
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
	return s.StoreSkipBlockInternal(psbd)
}

// StoreSkipBlockInternal bypasses the authentification performed in StoreSkipBlock.
// This method must be used in the case the service (like byzcoin) is running on the
// same host as the Skipchain one. If the Skipchain service is linked to a client,
// its behavior is to reject any new foreign resquests, even it it comes from a local
// service, like Byzcoin.
func (s *Service) StoreSkipBlockInternal(psbd *StoreSkipBlock) (*StoreSkipBlockReply, error) {
	err := s.incrementWorking()
	if err != nil {
		return nil, err
	}
	defer s.decrementWorking()

	// Initial checks on the proposed block.
	prop := psbd.NewBlock
	if len(prop.Roster.List) == 0 {
		return nil, errors.New("empty roster")
	}

	if !s.ServerIdentity().Equal(prop.Roster.Get(0)) {
		return nil, errors.New(
			"only leader is allowed to add blocks")
	}
	var prev *SkipBlock

	// If TargetSkipChainID is not given, it is a genesis block.
	if psbd.TargetSkipChainID.IsNull() {
		// A new chain is created
		log.Lvl2("Creating new skipchain with roster", psbd.NewBlock.Roster.List)
		prop.Height = prop.MaximumHeight
		prop.ForwardLink = make([]*ForwardLink, 0)
		// genesis block has a random back-link, so that two
		// identical genesis blocks have a different ID.
		var bl [32]byte
		random.Bytes(bl[:], random.New())
		prop.BackLinkIDs = []SkipBlockID{SkipBlockID(bl[:])}
		prop.GenesisID = nil
		// starting with release v3.1.0, new skipchains default to BDN
		// because BLS is vulnerable a known attack (see ../README.md
		// about the release).
		prop.SignatureScheme = BdnSignatureSchemeIndex
		prop.updateHash()
		err := s.verifyBlock(prop)
		if err != nil {
			return nil, err
		}

		// Propagate only the genesis block and let conodes ask for
		// missing data
		if err := s.startGenesisPropagation(prop); err != nil {
			return nil, errors.New(
				"Couldn't propagate new blocks: " + err.Error())
		}
	} else {

		// We're appending a block to an existing chain.
		log.Lvlf2("Preparing to add block with roster %+v to %x",
			psbd.NewBlock.Roster.List, psbd.TargetSkipChainID)

		// Check if we really want to take this block with regard to our
		// authorization lists.
		if !s.BlockIsFriendly(psbd.NewBlock) {
			log.Lvlf2("%s: block is not friendly: %x",
				s.ServerIdentity(), psbd.NewBlock.Hash)
			return nil, errors.New("chain not followed")
		}

		// At this point the TargetSkipChainID must have something in
		// it, so we look for the correct skipchain identified by it.
		// For backward compatibility, TargetSkipChainID does not need
		// to be a genesis block, it can also be a block on one of the
		// chains.
		// This makes sure we have the genesis block and get the correct
		// ID.
		scID := psbd.TargetSkipChainID
		sb := s.db.GetByID(psbd.TargetSkipChainID)
		if sb == nil {
			err := s.SyncChain(psbd.NewBlock.Roster, scID)
			if err != nil {
				return nil, errors.New("didn't find block to attach to: " + err.Error())
			}
			sb = s.db.GetByID(scID)
			if sb == nil {
				return nil, errors.New("couldn't update to latest block")
			}
		}
		if !sb.SkipChainID().Equal(scID) {
			// In case the user asked to add to a specific block, get the correct
			// skipchain-ID.
			scID = sb.SkipChainID()
		}

		// From now on we have everything we need and lock the adding of new blocks
		// from this leader to this skipchain.
		s.chains.lock(scID)
		defer s.chains.unlock(scID)

		var err error
		prev, err = s.db.GetLatestByID(scID)
		if err != nil {
			return nil, errors.New("error while getting latest block: " + err.Error())
		}

		// Check the roster of the previous block - for the protocols to work
		// correctly, we need to be in the roster of the latest block.
		if i, _ := prev.Roster.Search(s.ServerIdentity().ID); i < 0 {
			return nil, errors.New("this node is not in the previous roster")
		}

		// Check if the previous block already has a forward link.
		if len(prev.ForwardLink) > 0 {
			return nil, errors.New(
				"the latest block already has a follower")
		}

		// Copy the block-header to a new block.
		prop.MaximumHeight = prev.MaximumHeight
		prop.BaseHeight = prev.BaseHeight
		prop.VerifierIDs = prev.VerifierIDs
		prop.Index = prev.Index + 1
		prop.GenesisID = scID
		prop.ForwardLink = []*ForwardLink{}
		prop.SignatureScheme = prev.SignatureScheme
		// And calculate the height of that block.
		index := prop.Index
		for prop.Height = 1; index%prop.BaseHeight == 0; prop.Height++ {
			index /= prop.BaseHeight
			if prop.Height >= prop.MaximumHeight {
				break
			}
		}
		log.Lvl4("Found height", prop.Height, "for index", prop.Index,
			"and maxHeight", prop.MaximumHeight, "and base", prop.BaseHeight)

		// Add backlinks to the block.
		prop.BackLinkIDs = make([]SkipBlockID, prop.Height)
		pointer := prev
		for h := range prop.BackLinkIDs {
			// For every height, we pass the skiplist backwards at the lower height,
			// till we find a block with the desired height.
			for pointer.Height <= h {
				prevPointer := s.db.GetByID(pointer.BackLinkIDs[h-1])
				if prevPointer == nil {
					pp, err := s.getBlocks(pointer.Roster, pointer.BackLinkIDs[h-1], 1)
					if err != nil {
						return nil, errors.New("couldn't fetch missing block: " + err.Error())
					}
					if len(pp) == 0 {
						return nil, errors.New(
							"Didn't find convenient SkipBlock for height " +
								strconv.Itoa(h))
					}
					s.db.Store(pp[0])
					prevPointer = pp[0]
				}
				pointer = prevPointer
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

		// Create first forward link. The first forward link is crucial, as it's the
		// one where all the nodes will verify that the block is valid. Higher level
		// forward links can depend on this forward link.
		// After creating the forward link, it will propagate it to all nodes.
		if err := s.forwardLinkLevel0(prev, prop); err != nil {
			// As the block's creation failed, we need to clean the block buffer so
			// that other services know that no block are proposed.
			// This is done only on the leader side and then children won't be
			// notified until a different block is proposed.
			s.blockBuffer.clear(prev.SkipChainID())

			return nil, errors.New(
				"Couldn't get forward signature on block: " + err.Error())
		}

		if !s.disableForwardLink {
			// Now create all further forward links. Again, after creation of each
			// forward-link, it will propagate them to all nodes.
			log.Lvl3("Asking forward-links from all linked blocks")
			for i, bl := range prop.BackLinkIDs[1:] {
				back := s.db.GetByID(bl)
				if back == nil {
					return nil, errors.New(
						"Didn't get skipblock in back-link")

				}
				// Requesting creation of secondary forward link.
				log.Lvlf2("%s: sending request for height %d to %s", s.ServerIdentity(),
					i+1, back.Roster.List[0])
				// Deprecated: it should be replaced by the handler so that it can be checked
				// that the link has been created and try another node otherwise.
				err := s.SendRaw(back.Roster.List[0], &ForwardSignature{
					TargetHeight: i + 1,
					Previous:     back.Hash,
					Newest:       prop.Copy(),
				})

				if err != nil {
					log.Warn(err)
				}
			}
		}
	}
	reply := &StoreSkipBlockReply{
		Previous: prev,
		Latest:   prop,
	}
	log.Lvlf3("Block added, replying. New latest is: %x, at index %d", prop.Hash, prop.Index)
	return reply, nil
}

// sendForwardLinkRequest sends requests to conodes in the given roster until either the forward-link is
// created or there's not enough online nodes to get a valid signature.
func sendForwardLinkRequest(ro *onet.Roster, req *ForwardSignature, reply *ForwardSignatureReply) (err error) {
	cl := NewClient()

	// Try as many times as it can until the faulty threshold is reached
	// meaning it's impossible to get a valid signature.
	retries := protocol.DefaultFaultyThreshold(len(ro.List)) + 1

	for i := 0; i < retries; i++ {
		// No random permutation as we need a given threshold anyway.
		err = cl.SendProtobuf(ro.List[i], req, reply)
		if err == nil {
			return nil
		}
	}

	return err
}

// OptimizeProof creates missing forward links to optimize the proof of the block
// at the given index.
func (s *Service) OptimizeProof(req *OptimizeProofRequest) (*OptimizeProofReply, error) {
	pr, err := s.db.GetProofForID(req.ID)
	if err != nil {
		return nil, err
	}

	target := pr[len(pr)-1]
	index := 0
	h := 0
	newProof := Proof{}

	for _, sb := range pr[:len(pr)-1] {
		if sb.Index < index {
			// Skip blocks thanks to the new forward-link.
			continue
		}

		h, index = sb.pathForIndex(target.Index)

		if h > 0 && len(sb.ForwardLink) <= h {
			to := pr.Search(index)
			if to == nil {
				return nil, fmt.Errorf("chain is inconsistent: block at index %d not found", index)
			}

			req := &ForwardSignature{
				TargetHeight: h,
				Previous:     sb.Hash,
				Newest:       to,
			}
			reply := &ForwardSignatureReply{}

			log.Lvlf2("requesting missing forward-link at index %d with height %d / %d", sb.Index, h, index)
			// The signature must be created by the roster of the block
			err := sendForwardLinkRequest(sb.Roster, req, reply)

			if err != nil {
				log.Error("could not create a missing forward link:", err)
				// reset the index to try to create lower levels
				index = sb.Index
			} else {
				// save the new forward link
				err = sb.AddForwardLink(reply.Link, h)
				if err != nil {
					log.Error("could not store the missing forward-link:", err)
					index = sb.Index
				}
			}
		}

		newProof = append(newProof, sb)
	}

	newProof = append(newProof, target)

	// Propagate the optimized proof to the given roster
	err = s.startPropagation(s.propagateProof, req.Roster, &PropagateProof{newProof})

	return &OptimizeProofReply{newProof}, err
}

// GetUpdateChain returns a slice of SkipBlocks which describe the part of the
// skipchain from the latest block the caller knows to the latest
// SkipBlock we know. The last block in the returned slice of blocks is
// not guaranteed to have no forward links. It is up to the caller
// to continue following forward links with the new roster if necessary.
func (s *Service) GetUpdateChain(guc *GetUpdateChain) (*GetUpdateChainReply, error) {
	block := s.db.GetByID(guc.LatestID)
	if block == nil {
		return nil, errors.New("couldn't find latest skipblock")
	}

	blocks := []*SkipBlock{block.Copy()}
	log.Lvlf3("%s: starting to search chain at %x",
		s.Context.ServerIdentity(), guc.LatestID)
	maxHeight := guc.MaxHeight
	if maxHeight <= 0 {
		maxHeight = block.MaximumHeight
	}
	maxBlocks := guc.MaxBlocks
	// Loop for as long as we have available forward links and that we don't have
	// more than maxBlocks blocks - except if it is 0, then add as many blocks as
	// we have.
	for block.GetForwardLen() > 0 &&
		(maxBlocks <= 0 || len(blocks) < maxBlocks) {
		var link *ForwardLink
		if block.GetForwardLen() < maxHeight {
			link = block.ForwardLink[block.GetForwardLen()-1]
		} else {
			link = block.ForwardLink[maxHeight-1]
		}
		next := s.db.GetByID(link.To)
		if next == nil {
			// Next not found means that maybe the roster
			// has evolved and we are no longer aware of
			// this chain. The caller will be responsible
			// to issue a new GetUpdateChain with the
			// latest Roster to keep traversing.
			break
		}

		if next.Index <= block.Index {
			return nil, ErrorInconsistentForwardLink
		}

		block = next
		blocks = append(blocks, next.Copy())
	}
	log.Lvlf3("Found %d blocks", len(blocks))
	reply := &GetUpdateChainReply{Update: blocks}

	return reply, nil
}

// RegisterStoreSkipblockCallback sets a callback function in SkipBlockDB,
// which is called just before a skipblock is added/updated.
func (s *Service) RegisterStoreSkipblockCallback(f func(SkipBlockID) error) {
	s.db.callback = f
}

// SyncChain communicates with conodes in the Roster via getBlocks
// in order traverse the chain and save the blocks locally. It starts with
// the given 'latest' skipblockid and fetches all blocks up to the latest block.
// In case there is no link in the database to store the 'latest' skipblock,
// syncchain will start at the genesis block and fetch all blocks up to the latest
// skipblock. However, this means that the 'latest' skipblock might _not_ be in
// the database when SyncChain returns!
func (s *Service) SyncChain(roster *onet.Roster, latest SkipBlockID) error {
	// loop on getBlocks, fetching 10 at a time
	var allBlocks []*SkipBlock
	for {
		blocks, err := s.getBlocks(roster, latest, 10)
		if err != nil {
			return err
		}
		if len(blocks) == 0 {
			return errors.New("didn't find any corresponding blocks")
		}

		fBlock := blocks[0]
		if !s.db.HasForwardLink(fBlock) {
			if latest.Equal(fBlock.SkipChainID()) {
				return errors.New("synching failed even when trying to start at the genesis block")
			}
			log.Lvl3("couldn't store synched block - synching from genesis block")
			return s.SyncChain(fBlock.Roster, fBlock.SkipChainID())
		}
		allBlocks = append(allBlocks, blocks...)

		lBlock := blocks[len(blocks)-1]
		if len(lBlock.ForwardLink) == 0 {
			break
		}
		latest = lBlock.Hash
	}
	_, err := s.db.StoreBlocks(allBlocks)
	return err
}

// getBlocks uses ProtocolGetBlocks to return up to n blocks, traversing the
// skiplist forward from id. It contacts a random subgroup of some of the nodes
// in the roster, in order to find an answer, even in the case that a few
// nodes in the network are down.
func (s *Service) getBlocks(roster *onet.Roster, id SkipBlockID, n int) ([]*SkipBlock, error) {
	subCount := len(roster.List)
	if subCount > 10 {
		// Only take half of the nodes to not spam the whole network.
		subCount /= 2
	}
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
		return nil, err
	}
	select {
	case result := <-pisc.GetBlocksReply:
		if err := Proof(result).VerifyFromID(id); err != nil {
			return nil, err
		}
		return result, nil
	case <-time.After(s.propTimeout):
		pisc.Done()
		return nil, errors.New("timeout waiting for GetBlocks reply")
	case <-s.closing:
		pisc.Done()
		return nil, errors.New("closing")
	}
}

// getLastBlock talks one of the servers in roster in order to find the latest
// block that it knows about.
func (s *Service) getLastBlock(roster *onet.Roster, latest SkipBlockID) (*SkipBlock, error) {
	if _, si := roster.Search(s.ServerIdentity().ID); si != nil {
		sb := s.GetDB().GetByID(latest)
		if sb == nil {
			return nil, errors.New("doesn't have skipchain")
		}
		return s.GetDB().GetLatest(sb)
	}
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
		return nil, errors.New("No such block")

	}
	return sb, nil
}

// GetSingleBlockByIndex searches for the given block and returns it. If no such block is
// found, a nil is returned.
func (s *Service) GetSingleBlockByIndex(id *GetSingleBlockByIndex) (*GetSingleBlockByIndexReply, error) {
	sb := s.db.GetByID(id.Genesis)
	if sb == nil {
		return nil, errors.New("No such genesis-block")
	}
	links := []*ForwardLink{{
		To:        id.Genesis,
		NewRoster: sb.Roster,
	}}
	if sb.Index == id.Index {
		return &GetSingleBlockByIndexReply{sb, links}, nil
	}
	for len(sb.ForwardLink) > 0 {
		// Search for the highest ForwardLink that doesn't shoot over the target
		sb = func() *SkipBlock {
			for i := len(sb.ForwardLink) - 1; i >= 0; i-- {
				to := sb.ForwardLink[i].To
				// We can have holes in the forward links
				if to != nil {
					tmp := s.db.GetByID(to)
					if tmp != nil && tmp.Index <= id.Index {
						links = append(links, sb.ForwardLink[i])
						return tmp
					}
				}
			}
			return nil
		}()
		if sb == nil {
			return nil, errors.New("didn't find block in forward link")
		}
		if sb.Index == id.Index {
			return &GetSingleBlockByIndexReply{sb, links}, nil
		}
	}
	err := fmt.Errorf("no block with index \"%d\" found", id.Index)
	log.Error(s.ServerIdentity(), err)
	return nil, err
}

// GetAllSkipchains currently returns a list of all the known blocks.
// This is a bug, but for backwards compatibility it is being left as is.
//
// Deprecation warning: This function will be removed in v3.
//
// Caution: This could be a huge amount of data. This should only be
// used in diagnostic or test code where you know the size of the result
// that you expect.
func (s *Service) GetAllSkipchains(id *GetAllSkipchains) (*GetAllSkipchainsReply, error) {
	log.Warn("GetAllSkipchains is deprecated because it returns all blocks instead of all skipchains. Migrate to GetAllSkipChainIDs.")
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

// GetAllSkipChainIDs returns the SkipBlockIDs of the genesis blocks
// of all of the known skipchains.
func (s *Service) GetAllSkipChainIDs(id *GetAllSkipChainIDs) (*GetAllSkipChainIDsReply, error) {
	gen, err := s.db.getAllSkipchains()
	if err != nil {
		return nil, err
	}
	reply := &GetAllSkipChainIDsReply{IDs: make([]SkipBlockID, len(gen))}

	ct := 0
	for k := range gen {
		reply.IDs[ct] = SkipBlockID(k)
		ct++
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
	if add.Conode != nil {
		msg = append(msg, add.Conode.ID[:]...)
	}
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
							closing:  make(chan bool),
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
		roster := onet.NewRoster([]*network.ServerIdentity{add.Conode})
		last, err := s.getLastBlock(roster, add.SkipchainID)
		if err != nil {
			return nil, errors.New("couldn't lookup skipchain: " + err.Error())
		}
		s.Storage.Follow = append(s.Storage.Follow,
			FollowChainType{
				Block:    last,
				NewChain: add.NewChain,
				closing:  make(chan bool),
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

// WaitBlock returns a block by its ID instantly if already stored in the DB
// or check if the block is inside the buffer. If the block is known, false will
// be returned because a catch up is not necessary, true otherwise.
func (s *Service) WaitBlock(sid SkipBlockID, id SkipBlockID) (*SkipBlock, bool) {
	sb := s.db.GetByID(id)
	if sb != nil {
		return sb, false
	}

	ok := s.blockBuffer.has(sid)
	if ok {
		return nil, false
	}

	return nil, true
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

// SetBFTTimeout can be used in tests to change the timeout passed
// to BFTCoSi.
func (s *Service) SetBFTTimeout(t time.Duration) {
	s.bftTimeout = t
}

// SetPropTimeout is used to set the propagation timeout.
func (s *Service) SetPropTimeout(t time.Duration) {
	s.propTimeout = t
}

// TestClose is called by Server.Close in case we're in testing. It
// makes sure that skipchain is not processing requests and will avoid
// further requests that might be queued up.
func (s *Service) TestClose() {
	s.closedMutex.Lock()
	if !s.closed {
		s.closed = true
		for _, fct := range s.Storage.Follow {
			fct.Shutdown()
		}
		close(s.closing)
		s.closedMutex.Unlock()
		s.working.Wait()
	} else {
		s.closedMutex.Unlock()
	}
}

// TestRestart stops and starts the service, initializing the skipchain-service
// structure.
func (s *Service) TestRestart() error {
	s.TestClose()
	db, bucket := s.GetAdditionalBucket([]byte("skipblocks"))
	s.db = NewSkipBlockDB(db, bucket)
	s.Storage = &Storage{}
	// Don't reset the verifiers, keep them
	//s.verifiers = map[VerifierID]SkipBlockVerifier{}
	s.propTimeout = defaultPropagateTimeout
	s.blockBuffer = newSkipBlockBuffer()
	s.closed = false
	s.closing = make(chan bool)
	return s.tryLoad()
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

// forwardLinkLevel0 is used to add a new block to the skipchain.
// It verifies if the new block is valid. If it is not valid, it
// returns with an error.
// If it finds a valid block, a BFT-signature is requested and the
// forward-link will be added.
// This only works for the level-0 forward link, that is, to link
// from the latest to the new block. For higher level links, less
// verifications need to be done using forwardLink.
func (s *Service) forwardLinkLevel0(src, dst *SkipBlock) error {
	err := s.incrementWorking()
	if err != nil {
		return err
	}
	defer s.decrementWorking()

	if src.GetForwardLen() > 0 {
		return errors.New("already have forward-link at this height")
	}

	// create the message we want to sign for this round
	roster := src.Roster

	log.Lvlf2("%s is adding forward-link level 0 to: %d->%d with roster %s", s.ServerIdentity(),
		src.Index, dst.Index, roster.List)
	fs := &ForwardSignature{
		TargetHeight: 0,
		Previous:     src.Hash,
		Newest:       dst,
	}
	data, err := network.Marshal(fs)
	if err != nil {
		return fmt.Errorf("Couldn't marshal block: %s", err.Error())
	}
	fwd := NewForwardLink(src, dst)
	protoName, _ := src.SignatureProtocol()
	sig, err := s.startBFT(protoName, roster, dst.Roster, fwd.Hash(), data)
	if err != nil {
		log.Error(s.ServerIdentity().Address, "startBFT failed with", err)
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

	// We send the new forward link to the previous roster only
	err = s.startPropagation(s.propagateForwardLink, roster, &PropagateForwardLink{fwd, 0})
	if err != nil {
		log.Error("Failed to propagate the forward link to the previous roster:", err)
	}

	// We send the shortest chain to the new conodes to let
	// them know they joined the cothority
	proof, err := s.db.GetProof(src.SkipChainID())
	if err != nil {
		return err
	}

	newRoster := []*network.ServerIdentity{}
	for _, si := range dst.Roster.List {
		if i, _ := src.Roster.Search(si.ID); i < 0 {
			newRoster = append(newRoster, si)
		}
	}

	if len(newRoster) == 0 {
		return nil
	}

	log.Lvlf3("%v is propagating %d blocks to %v", s.ServerIdentity(), len(proof), newRoster)

	// current conode needs to be in the propagation roster
	newRoster = append(newRoster, s.ServerIdentity())
	return s.startPropagation(s.propagateProof, onet.NewRoster(newRoster), &PropagateProof{proof})
}

// bftForwardLinkLevel0 makes sure that a signature-request for a forward-link
// is valid.
func (s *Service) bftForwardLinkLevel0(msg, data []byte) bool {
	log.Lvlf4("%s verifying block %x", s.ServerIdentity(), msg)
	_, fsInt, err := network.Unmarshal(data, cothority.Suite)
	if err != nil {
		log.Error(s.ServerIdentity().Address, "Couldn't unmarshal ForwardSignature", data)
		return false
	}
	fs, ok := fsInt.(*ForwardSignature)
	if !ok {
		log.Errorf("got unexpected type %T", fsInt)
		return false
	}
	log.Lvlf2("%s asked to sign forward-link to block %d : %x from %x",
		s.ServerIdentity(), fs.Newest.Index, fs.Newest.Hash, fs.Previous)
	if fs.TargetHeight != 0 {
		log.Errorf("got unexpected target height: %d", fs.TargetHeight)
		return false
	}

	prevSB := s.db.GetByID(fs.Previous)
	if prevSB == nil {
		if !s.BlockIsFriendly(fs.Newest) {
			log.Lvlf2("%s: block is not friendly: %x", s.ServerIdentity(), fs.Newest.Hash)
			return false
		}
		log.Lvl2(s.ServerIdentity(), "Didn't find src-skipblock, trying to sync")
		if err := s.SyncChain(fs.Newest.Roster, fs.Previous); err != nil {
			log.Error("failed to sync skipchain:", err)
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
		for i, verifier := range prevSB.VerifierIDs {
			if !verifier.Equal(fs.Newest.VerifierIDs[i]) {
				log.Lvlf2("Verifier IDs in the forward signature is wrong: %s != %s", verifier.String(), fs.Newest.VerifierIDs[i].String())
				return false
			}
		}
		for _, ver := range fs.Newest.VerifierIDs {
			f, exists := s.verifiers[ver]
			if !exists {
				log.Lvlf2("Found no user verification for %s", ver)
				return false
			}
			// Now we call the verification function. Wrap up f() inside of
			// g(), so that we can recover panics from f().
			g := func(to []byte, newest *SkipBlock) (out bool) {
				defer func() {
					if re := recover(); re != nil {
						log.Error("Verification function panic: " + re.(string))
						out = false
					}
				}()
				out = f(to, newest)
				return
			}

			if !g(fl.To, fs.Newest) {
				fname := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
				log.Lvlf2("Verification function failed: %v %s", fname, ver)
				return false
			}
		}
		return true
	}()
	if ok {
		s.verifyNewBlockBuffer.Store(sliceToArr(msg), true)
		// We can cache the block because it has been verified by this conode
		// but it will be added to the DB later on after the protocol
		// has succeeded
		s.blockBuffer.add(fs.Newest)
	}
	return ok
}

func (s *Service) bftForwardLinkLevel0Ack(msg []byte, data []byte) bool {
	arr := sliceToArr(msg)
	_, ok := s.verifyNewBlockBuffer.Load(arr)
	if ok {
		s.verifyNewBlockBuffer.Delete(arr)
	} else {
		_, fsInt, err := network.Unmarshal(data, cothority.Suite)
		if err != nil {
			log.Error(s.ServerIdentity().Address, "Couldn't unmarshal ForwardSignature", data)
			return false
		}
		fs, ok := fsInt.(*ForwardSignature)
		if !ok {
			log.Errorf("got unexpected type %T", fsInt)
			return false
		}
		log.Errorf("%s refuses to acknowledge unknown forward-link to block %d : %x from %x",
			s.ServerIdentity(), fs.Newest.Index, fs.Newest.Hash, fs.Previous)
	}
	return ok
}

// forwardLink receives a signature request of a newly accepted block.
// It only needs the 2nd-newest block and the forward-link.
func (s *Service) forwardLink(req *network.Envelope) error {
	fsOrig, ok := req.Msg.(*ForwardSignature)
	if !ok {
		return errors.New("didn't get ForwardSignature message")
	}

	_, err := s.ForwardLinkHandler(fsOrig)
	return err
}

// ForwardLinkHandler receives a forward-link signature request for a block already
// appended to the chain (height >= 1).
func (s *Service) ForwardLinkHandler(req *ForwardSignature) (*ForwardSignatureReply, error) {
	err := s.incrementWorking()
	if err != nil {
		return nil, err
	}
	defer s.decrementWorking()

	// Copy to prevent data race when the message is sent to itself.
	fs := &ForwardSignature{
		TargetHeight: req.TargetHeight,
		Previous:     req.Previous,
		Newest:       req.Newest.Copy(),
		Links:        make([]*ForwardLink, 0),
	}

	fl, err := func() (*ForwardLink, error) {
		if fs.TargetHeight >= len(fs.Newest.BackLinkIDs) {
			return nil, fmt.Errorf("This backlink-height doesn't exist for block at index %d: %d / %d",
				fs.Newest.Index, fs.TargetHeight, len(fs.Newest.BackLinkIDs))
		}
		from := s.db.GetByID(fs.Newest.BackLinkIDs[fs.TargetHeight])
		if from == nil {
			return nil, errors.New("Didn't find target-block")
		}
		if !fs.Previous.Equal(from.Hash) {
			return nil, errors.New("TargetHeight backlink doesn't correspond to previous")
		}
		// Add links to prove the newest block is valid.
		pointer := from
		for !pointer.Hash.Equal(fs.Newest.Hash) {
			if len(pointer.ForwardLink) == 0 {
				err := s.SyncChain(pointer.Roster, pointer.Hash)
				if err != nil {
					return nil, err
				}

				pointer = s.db.GetByID(pointer.Hash)
				if pointer == nil || len(pointer.ForwardLink) == 0 {
					return nil, errors.New("Couldn't reach the proposed block from the backlink")
				}
			}
			highest := pointer.ForwardLink[len(pointer.ForwardLink)-1]
			fs.Links = append(fs.Links, highest)
			next := s.db.GetByID(highest.To)
			if next == nil {
				sbs, err := s.getBlocks(pointer.Roster, highest.To, 1)
				if err != nil || len(sbs) == 0 {
					return nil, errors.New("cannot create proof that the blocks are linked: " + err.Error())
				}
				next = sbs[0]
			}
			pointer = next
		}
		data, err := network.Marshal(fs)
		if err != nil {
			return nil, err
		}
		fl := NewForwardLink(from, fs.Newest)
		_, protoName := from.SignatureProtocol()
		sig, err := s.startBFT(protoName, from.Roster, fs.Newest.Roster, fl.Hash(), data)
		if err != nil {
			return nil, errors.New("Couldn't get signature: " + err.Error())
		}
		log.Lvl2("Adding forward-link level", fs.TargetHeight, "to block", from.Index)

		fl.Signature = *sig
		if !from.Roster.ID.Equal(fs.Newest.Roster.ID) {
			fl.NewRoster = fs.Newest.Roster
		}
		if err = from.AddForwardLink(fl, fs.TargetHeight); err != nil {
			return nil, err
		}

		// Forward-links are sent to the new roster so active conodes get the update. If a conode
		// is exluded from the cothority, it will need to catch up the forward link later when
		// re-entering the cothority.
		ro := fs.Newest.Roster.Concat(s.ServerIdentity())
		return fl, s.startPropagation(s.propagateForwardLink, ro, &PropagateForwardLink{fl, fs.TargetHeight})
	}()
	if err != nil {
		return nil, fmt.Errorf("%v couldn't create forwardLink: %v", s.ServerIdentity(), err)
	}
	return &ForwardSignatureReply{Link: fl}, nil
}

// verifyFollowBlock makes sure that a signature-request for a forward-link
// is valid.
func (s *Service) bftForwardLink(msg, data []byte) bool {
	err := func() error {
		_, fsInt, err := network.Unmarshal(data, cothority.Suite)
		if err != nil {
			return err
		}
		fs, ok := fsInt.(*ForwardSignature)
		if !ok {
			return errors.New("didn't receive a ForwardSignature")
		}

		// Retrieve the src and dst blocks and make sure the basic parameters
		// are ok.
		dst := fs.Newest
		if dst == nil || !dst.CalculateHash().Equal(dst.Hash) {
			return errors.New("newest block does not match its hash")
		}
		if fs.TargetHeight > len(dst.BackLinkIDs) || fs.TargetHeight < 0 {
			return errors.New("unexpected target height value")
		}
		src := s.db.GetByID(dst.BackLinkIDs[fs.TargetHeight])
		if src == nil {
			return errors.New("don't have src-block")
		}
		h, index := src.pathForIndex(dst.Index)
		if dst.Index != index || fs.TargetHeight != h {
			return errors.New("target height does not match the newest block")
		}
		if src.GetForwardLen() >= fs.TargetHeight+1 {
			return errors.New("already have forward-link at height " +
				strconv.Itoa(fs.TargetHeight+1))
		}
		if !src.SkipChainID().Equal(dst.SkipChainID()) {
			return errors.New("src and newest not from same skipchain")
		}

		// Make sure the links are correctly linking src to dst:
		// - every link is correctly linked to the previous and next link
		// - the signatures are correct
		if len(fs.Links) == 0 {
			return errors.New("link list should not be empty")
		}

		newRoster := src.Roster

		for i, fl := range fs.Links {
			publics := newRoster.ServicePublics(ServiceName)

			if err := fl.VerifyWithScheme(suite, publics, src.SignatureScheme); err != nil {
				return errors.New("verification failed: " + err.Error())
			}
			if fl.NewRoster != nil {
				newRoster = fl.NewRoster
			}
			if i == 0 {
				if !src.Hash.Equal(fl.From) {
					return errors.New("first link in link list is not source-block")
				}
			} else {
				if !fl.From.Equal(fs.Links[i-1].To) {
					return errors.New("links are not correctly chained together")
				}
			}
		}
		if !fs.Links[len(fs.Links)-1].To.Equal(dst.Hash) {
			return errors.New("latest link doesn't point to newest block")
		}

		// Verify the forward link itself is correct before agreeing to sign
		// it.
		fl := NewForwardLink(src, fs.Newest)
		if bytes.Compare(fl.Hash(), msg) != 0 {
			return errors.New("hash to sign doesn't correspond to ForwardSignature")
		}
		return nil
	}()
	if err != nil {
		log.Error(err)
		return false
	}

	s.verifyFollowBlockBuffer.Store(sliceToArr(msg), true)
	return true
}

func (s *Service) bftForwardLinkAck(msg, data []byte) bool {
	arr := sliceToArr(msg)
	_, ok := s.verifyFollowBlockBuffer.Load(arr)
	if ok {
		s.verifyFollowBlockBuffer.Delete(arr)
	} else {
		log.Errorf("%s ack failed for msg %x", s.ServerIdentity().Address, msg)
	}
	return ok
}

// startBFT starts a BFT-protocol with the given parameters. We can only start
// the bft protocol if we're the root. The origRoster is the roster that should
// be used to used in the normal case. newRoster is an optimisation that will
// be used if the ID between the two rosters are different but the aggregate is
// the same. This is an optimisation because the newer roster might have an
// order that is more likely to give us non-failing subleaders in the byzcoinx
// protocol.
func (s *Service) startBFT(proto string, origRoster, newRoster *onet.Roster, msg, data []byte) (*byzcoinx.FinalSignature, error) {
	// Before BDN signatures, the new roster was used when it was a rotation so
	// that subleaders were more likely to be alive. It doesn't work anymore with
	// BDN signatures because the way coefficients are computed.
	// Instead the co-signing protocol has been modified to generate the tree by
	// taking in account the position of the root in the roster to assign subleaders
	// that are more likely alive.
	roster := origRoster

	if len(roster.List) == 0 {
		return nil, errors.New("found empty Roster")
	}

	// Start the protocol
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
	root := node.(*byzcoinx.ByzCoinX)

	// Register the function generating the protocol instance
	root.Msg = msg
	root.Data = data
	root.CreateProtocol = s.CreateProtocol
	root.FinalSignatureChan = make(chan byzcoinx.FinalSignature, 1)
	root.Timeout = s.propTimeout
	root.Threshold = byzcoinx.Threshold(len(tree.List()))
	if s.bftTimeout != 0 {
		root.Timeout = s.bftTimeout
	}

	log.Lvl3(s.ServerIdentity(), "starts bft-cosi")
	if err := node.Start(); err != nil {
		log.Error("failed to start with error", err)
		return nil, err
	}

	select {
	case sig := <-root.FinalSignatureChan:
		if sig.Sig == nil {
			return nil, errors.New("couldn't sign forward-link")
		}
		log.Lvl3(s.ServerIdentity(), "bft-cosi done")

		return &sig, nil
	case <-time.After(root.Timeout * 2):
		return nil, errors.New("timed out while waiting for signature")
	case <-s.closing:
		return nil, errors.New("closing down")
	}
}

// propagateGenesisHandler will save a new SkipBlock
func (s *Service) propagateGenesisHandler(msg network.Message) error {
	pg, ok := msg.(*PropagateGenesis)
	if !ok {
		return errors.New("Couldn't convert to slice of SkipBlocks")
	}

	if !s.BlockIsFriendly(pg.Genesis) {
		return errors.New("Conode doesn't want to follow that skipchain")
	}

	id := s.db.Store(pg.Genesis)
	if id == nil {
		return errors.New("failed to store the block")
	}
	return nil
}

// propagateForwardLinkHandler will update the latest block with
// the new forward link and the new block when given
func (s *Service) propagateForwardLinkHandler(msg network.Message) error {
	s.closedMutex.Lock()
	defer s.closedMutex.Unlock()
	if s.closed {
		return xerrors.New("service is closed")
	}
	pfl, ok := msg.(*PropagateForwardLink)
	if !ok {
		return xerrors.New("couldn't convert to a ForwardLink propagation")
	}

	// Get the block to update the list of FLs
	sb := s.db.GetByID(pfl.ForwardLink.From)
	if sb == nil {
		// Here we assume the block must be there because it should
		// have caught up during the signature request
		return xerrors.New("couldn't get the block to attach the forward link")
	}
	log.Lvlf2("Adding Forwardlink to block %d: (%x height:%d %x)",
		sb.Index, pfl.Height, pfl.ForwardLink.From, pfl.ForwardLink.To)

	err := sb.AddForwardLink(pfl.ForwardLink, pfl.Height)
	if err != nil {
		return xerrors.Errorf("couldn't add forward-link: %v", err)
	}

	blocks := []*SkipBlock{sb}

	if pfl.Height == 0 {
		newBlock := s.blockBuffer.get(sb.SkipChainID(), pfl.ForwardLink.To)
		if newBlock == nil {
			return xerrors.New("cannot store forward-link if there is no" +
				" corresponding block")
		}
		blocks = append(blocks, newBlock)

		// The buffer needs to be cleared only once the new block has been
		// stored, else byzcoin will think the block is missing.
		defer func() {
			log.Lvl2("Clearing block")
			s.blockBuffer.clear(sb.SkipChainID())
		}()
	}

	// Update the forward link of the previous latest block and add the new
	// block.
	log.Lvl2("Storing new forward-link and eventual new block")
	if _, err := s.db.StoreBlocks(blocks); err != nil {
		return xerrors.Errorf("error while storing forward-link and new block"+
			": %v", err)
	}

	return nil
}

// PropagateProof is a simple function that will build the proof of a given
// skipchain and send it the given roster.
func (s *Service) PropagateProof(roster *onet.Roster, sid SkipBlockID) error {
	proof, err := s.db.GetProof(sid)
	if err != nil {
		return err
	}

	// The propagation protocol expect this server to be present in the roster.
	rosterWithRoot := roster.Concat(s.ServerIdentity())

	return s.startPropagation(s.propagateProof, rosterWithRoot, &PropagateProof{proof})
}

// propagateProofHandler handles a chain propagation message that
// announces a skipchain to a new conode
func (s *Service) propagateProofHandler(msg network.Message) error {
	pc, ok := msg.(*PropagateProof)
	if !ok {
		return errors.New("Couldn't convert to PropagateProof message")
	}

	if len(pc.Proof) > 0 && !s.BlockIsFriendly(pc.Proof[0]) {
		return errors.New("Block is not friendly")
	}

	if err := pc.Proof.Verify(); err != nil {
		return fmt.Errorf("Proof verification failed with: %s", err.Error())
	}

	_, err := s.db.StoreBlocks(pc.Proof)
	if err != nil {
		return err
	}

	log.Lvlf3("Proof has been propagated to %v", s.ServerIdentity())
	return nil
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func (s *Service) registerVerification(v VerifierID, f SkipBlockVerifier) error {
	s.verifiers[v] = f
	return nil
}

// verifyBlock makes sure the basic parameters of a block are correct and returns
// an error if something fails.
func (s *Service) verifyBlock(sb *SkipBlock) error {
	if sb.MaximumHeight <= 0 {
		return errors.New("Set a maximumHeight > 0")
	}
	if sb.BaseHeight <= 0 {
		return errors.New("Set a baseHeight > 0")
	}
	if sb.BaseHeight == 1 && sb.MaximumHeight > 1 {
		return errors.New("Set maximumHeight to 1 when baseHeight is 1")
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

func (s *Service) startPropagation(propagate messaging.PropagationFunc, ro *onet.Roster, msg network.Message) error {
	err := s.incrementWorking()
	if err != nil {
		return err
	}
	defer s.decrementWorking()

	replies, err := propagate(ro, msg, s.propTimeout)
	if err != nil {
		return err
	}

	if replies != len(ro.List) {
		log.Lvl1(s.ServerIdentity(), "Only got", replies, "out of", len(ro.List))
	}

	return nil
}

// notify other services about new/updated skipblock
func (s *Service) startGenesisPropagation(genesis *SkipBlock) error {
	roster := genesis.Roster
	log.Lvlf3("%s: propagating %x to %s", s.ServerIdentity(), genesis.Hash, roster.List)

	return s.startPropagation(s.propagateGenesis, roster, &PropagateGenesis{genesis})
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

// ChainIsFriendly searches if this chain should be allowed or not.
func (s *Service) ChainIsFriendly(scID SkipBlockID) bool {
	s.storageMutex.Lock()
	defer s.storageMutex.Unlock()

	// If no skipchains are stored, allow everything
	if len(s.Storage.FollowIDs) == 0 {
		return true
	}
	// accept all blocks that are already stored with us.
	if s.db.GetByID(scID) != nil {
		return true
	}
	// Also accept blocks that are stored in the FollowIDs
	for _, id := range s.Storage.FollowIDs {
		if id.Equal(scID) {
			return true
		}
	}
	return false
}

// BlockIsFriendly searches if all members of the new block are followed
// by this node.
func (s *Service) BlockIsFriendly(sb *SkipBlock) bool {
	if s.ChainIsFriendly(sb.SkipChainID()) {
		return true
	}
	// Accept if we're the root.
	index, _ := sb.Roster.Search(s.ServerIdentity().ID)
	if index == 0 {
		return true
	}

	// If no skipchains are stored, allow everything
	if len(s.Storage.Follow) == 0 {
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
	s.SaveVersion(dbVersion)
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
	for i := range s.Storage.Follow {
		s.Storage.Follow[i].closing = make(chan bool)
	}
	return nil
}

// sliceToArr does what the name suggests, we need it to turn a slice into
// something hashable.
func sliceToArr(msg []byte) [32]byte {
	var arr [32]byte
	copy(arr[:], msg)
	return arr
}

func newSkipchainService(c *onet.Context) (onet.Service, error) {
	db, bucket := c.GetAdditionalBucket([]byte("skipblocks"))
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		db:               NewSkipBlockDB(db, bucket),
		Storage:          &Storage{},
		verifiers:        map[VerifierID]SkipBlockVerifier{},
		propTimeout:      defaultPropagateTimeout,
		closing:          make(chan bool),
		blockBuffer:      newSkipBlockBuffer(),
	}

	if err := s.tryLoad(); err != nil {
		return nil, err
	}
	log.ErrFatal(s.RegisterHandlers(s.StoreSkipBlock, s.GetUpdateChain,
		s.GetSingleBlock, s.GetSingleBlockByIndex, s.GetAllSkipchains,
		s.GetAllSkipChainIDs, s.OptimizeProof,
		s.CreateLinkPrivate, s.Unlink, s.AddFollow, s.ListFollow,
		s.DelFollow, s.Listlink, s.ForwardLinkHandler))
	s.ServiceProcessor.RegisterStatusReporter("Skipblock", s.db)
	// Deprecated: the handler should be used instead
	s.RegisterProcessorFunc(network.RegisterMessage(&ForwardSignature{}), s.forwardLink)

	if err := s.registerVerification(VerifyBase, s.verifyFuncBase); err != nil {
		return nil, err
	}

	var err error
	s.propagateGenesis, err = messaging.NewPropagationFunc(c, "SkipchainPropagate", s.propagateGenesisHandler, -1)
	if err != nil {
		return nil, err
	}
	s.propagateForwardLink, err = messaging.NewPropagationFunc(c, "SkipchainPropagateFL", s.propagateForwardLinkHandler, -1)
	if err != nil {
		return nil, err
	}
	s.propagateProof, err = messaging.NewPropagationFunc(c, "SkipchainPropagateProof", s.propagateProofHandler, -1)
	if err != nil {
		return nil, err
	}
	// Register ByzCoinX protocols for BLS
	err = byzcoinx.InitBFTCoSiProtocol(suite, s.Context,
		s.bftForwardLinkLevel0, s.bftForwardLinkLevel0Ack, bftNewBlock)
	if err != nil {
		return nil, err
	}
	err = byzcoinx.InitBFTCoSiProtocol(suite, s.Context,
		s.bftForwardLink, s.bftForwardLinkAck, bftFollowBlock)
	if err != nil {
		return nil, err
	}
	// Register ByzCoinX protocols for BDN
	err = byzcoinx.InitBDNCoSiProtocol(suite, s.Context,
		s.bftForwardLinkLevel0, s.bftForwardLinkLevel0Ack, bdnNewBlock)
	if err != nil {
		return nil, err
	}
	err = byzcoinx.InitBDNCoSiProtocol(suite, s.Context,
		s.bftForwardLink, s.bftForwardLinkAck, bdnFollowBlock)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// decrementWorking announces the end of a routine
func (s *Service) decrementWorking() {
	s.working.Done()
}

// incrementWorking increases the WaitGroup used to
// synchronize a close call to wait for any work
// to be done (e.g. tests)
func (s *Service) incrementWorking() error {
	s.closedMutex.Lock()
	if s.closed {
		s.closedMutex.Unlock()
		return errors.New("closing down")
	}
	s.working.Add(1)
	s.closedMutex.Unlock()

	return nil
}
