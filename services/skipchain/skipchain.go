package skipchain

import (
	"crypto/rand"
	"errors"

	"bytes"

	"sync"

	"strconv"

	"time"

	"io/ioutil"
	"os"

	"fmt"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/bftcosi"
	"github.com/dedis/cothority/protocols/manage"
	"github.com/dedis/cothority/sda"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "Skipchain"
const skipchainBFT = "SkipchainBFT"

func init() {
	sda.RegisterNewService(ServiceName, newSkipchainService)
	skipchainSID = sda.ServiceFactory.ServiceID(ServiceName)
	sda.GlobalProtocolRegister(skipchainBFT, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, nil)
	})
	network.RegisterPacketType(&SkipBlockMap{})
}

// XXX Why skipchainSID is private ? Should we not be able to access it from
// outside ?
var skipchainSID sda.ServiceID

// Service handles adding new SkipBlocks
type Service struct {
	*sda.ServiceProcessor
	// SkipBlocks points from SkipBlockID to SkipBlock but SkipBlockID is not a valid
	// key-type for maps, so we need to cast it to string
	*SkipBlockMap
	gMutex    sync.Mutex
	path      string
	verifiers map[VerifierID]SkipBlockVerifier

	// testVerify is set to true if a verification happened - only for testing
	testVerify bool
}

// SkipBlockMap holds the map to the skipblocks so it can be marshaled.
type SkipBlockMap struct {
	SkipBlocks map[string]*SkipBlock
}

// ProposeSkipBlock takes a hash for the latest valid SkipBlock and a SkipBlock
// that will be verified. If the verification returns true, the new SkipBlock
// will be signed and added to the chain and returned.
// If the the latest block given is nil it verify if we are actually creating
// the first (genesis) block and creates it. If it is called with nil although
// there already exist previous blocks, it will return an error.
func (s *Service) ProposeSkipBlock(si *network.ServerIdentity, psbd *ProposeSkipBlock) (network.Body, error) {
	prop := psbd.Proposed
	var prev *SkipBlock

	if !psbd.LatestID.IsNull() {
		// We're appending a block to an existing chain
		var ok bool
		prev, ok = s.getSkipBlockByID(psbd.LatestID)
		if !ok {
			return nil, errors.New("Didn't find latest block")
		}
		prop.MaximumHeight = prev.MaximumHeight
		prop.BaseHeight = prev.BaseHeight
		prop.ParentBlockID = prev.ParentBlockID
		prop.VerifierID = prev.VerifierID
		prop.Index = prev.Index + 1
		index := prop.Index
		for prop.Height = 1; index%prop.BaseHeight == 0; prop.Height++ {
			index /= prop.BaseHeight
			if prop.Height >= prop.MaximumHeight {
				break
			}
		}
		log.Lvl4("Found height", prop.Height, "for index", prop.Index,
			"and maxHeight", prop.MaximumHeight, "and base", prop.BaseHeight)
		prop.BackLinkIds = make([]SkipBlockID, prop.Height)
		pointer := prev
		for h := range prop.BackLinkIds {
			for pointer.Height < h+1 {
				var ok bool
				pointer, ok = s.getSkipBlockByID(pointer.BackLinkIds[0])
				if !ok {
					return nil, errors.New("Didn't find convenient SkipBlock for height " +
						strconv.Itoa(h))
				}
			}
			prop.BackLinkIds[h] = pointer.Hash
		}
	} else {
		// A new chain is created, suppose all arguments in SkipBlock
		// are correctly set up
		prop.Index = 0
		if prop.MaximumHeight == 0 {
			return nil, errors.New("Set a maximumHeight > 0")
		}
		if prop.BaseHeight == 0 {
			return nil, errors.New("Set a baseHeight > 0")
		}
		prop.Height = prop.MaximumHeight
		prop.ForwardLink = make([]*BlockLink, 0)
		// genesis block has a random back-link:
		bl := make([]byte, 32)
		rand.Read(bl)
		prop.BackLinkIds = []SkipBlockID{SkipBlockID(bl)}
	}
	if prop.Roster != nil {
		prop.Aggregate = prop.Roster.Aggregate
	}
	el, err := prop.GetResponsible(s)
	if err != nil {
		return nil, err
	}
	prop.AggregateResp = el.Aggregate

	prop.updateHash()

	prev, prop, err = s.signNewSkipBlock(prev, prop)
	if err != nil {
		return nil, errors.New("Verification error: " + err.Error())
	}
	s.save()

	reply := &ProposedSkipBlockReply{
		Previous: prev,
		Latest:   prop,
	}
	return reply, nil
}

// GetUpdateChain returns a slice of SkipBlocks which describe the part of the
// skipchain from the latest block the caller knows of to the actual latest
// SkipBlock.
// Somehow comparable to search in SkipLists.
func (s *Service) GetUpdateChain(si *network.ServerIdentity, latestKnown *GetUpdateChain) (network.Body, error) {
	block, ok := s.getSkipBlockByID(latestKnown.LatestID)
	if !ok {
		return nil, errors.New("Couldn't find latest skipblock")
	}
	// at least the latest know and the next block:
	blocks := []*SkipBlock{block}
	log.Lvl3("Starting to search chain")
	for len(block.ForwardLink) > 0 {
		link := block.ForwardLink[len(block.ForwardLink)-1]
		block, ok = s.getSkipBlockByID(link.Hash)
		if !ok {
			return nil, errors.New("Missing block in forward-chain")
		}
		blocks = append(blocks, block)
	}
	log.Lvl3("Found", len(blocks), "blocks")
	reply := &GetUpdateChainReply{blocks}

	return reply, nil
}

// SetChildrenSkipBlock creates a new SkipChain if that 'service' doesn't exist
// yet.
func (s *Service) SetChildrenSkipBlock(si *network.ServerIdentity, scsb *SetChildrenSkipBlock) (network.Body, error) {
	parentID := scsb.ParentID
	childID := scsb.ChildID
	parent, ok := s.getSkipBlockByID(parentID)
	if !ok {
		return nil, errors.New("Couldn't find skipblock!")
	}
	child, ok := s.getSkipBlockByID(childID)
	if !ok {
		return nil, errors.New("Couldn't find skipblock!")
	}
	child.ParentBlockID = parentID
	parent.ChildSL = NewBlockLink()
	parent.ChildSL.Hash = childID

	err := s.startPropagation([]*SkipBlock{child, parent})
	if err != nil {
		return nil, err
	}
	// Parent-block is always of type roster, but child-block can be
	// data or roster.
	reply := &SetChildrenSkipBlockReply{parent, child}
	s.save()

	return reply, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	var pi sda.ProtocolInstance
	var err error
	switch tn.ProtocolName() {
	case "Propagate":
		pi, err = manage.NewPropagateProtocol(tn)
		if err != nil {
			return nil, err
		}
		pi.(*manage.Propagate).RegisterOnData(s.PropagateSkipBlock)
	case skipchainBFT:
		pi, err = bftcosi.NewBFTCoSiProtocol(tn, s.bftVerify)
	}
	return pi, err
}

// PropagateSkipBlock will save a new SkipBlock
func (s *Service) PropagateSkipBlock(msg network.Body) {
	sb, ok := msg.(*SkipBlock)
	if !ok {
		log.Error("Couldn't convert to SkipBlock")
		return
	}
	if err := sb.VerifySignatures(); err != nil {
		log.Error(err)
		return
	}
	s.storeSkipBlock(sb)
	log.Lvlf3("Stored skip block %+v in %x", *sb, s.Context.ServerIdentity().ID[0:8])
}

// VerifyShardFunc makes sure that the cothority of the child-skipchain is
// part of the root-cothority.
func (s *Service) VerifyShardFunc(msg []byte, sb *SkipBlock) bool {
	if sb.ParentBlockID.IsNull() {
		log.Lvl3("No parent skipblock to verify against")
		return false
	}
	sbParent, exists := s.getSkipBlockByID(sb.ParentBlockID)
	if !exists {
		log.Lvl3("Parent skipblock doesn't exist")
		return false
	}
	for _, e := range sb.Roster.List {
		if i, _ := sbParent.Roster.Search(e.ID); i < 0 {
			log.Lvl3("ServerIdentity in child doesn't exist in parent")
			return false
		}
	}
	return true
}

// VerifyNoneFunc returns always true.
func (s *Service) VerifyNoneFunc(msg []byte, sb *SkipBlock) bool {
	log.Lvl4("No verification - accepted")
	return true
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func (s *Service) RegisterVerification(v VerifierID, f SkipBlockVerifier) error {
	s.verifiers[v] = f
	return nil
}

// signNewSkipBlock should start a BFT-signature on the newest block
// which will propagate and update all forward-links of all blocks.
// As a simple solution it verifies the validity of the block,
// simulates a signature and propagates the latest and newest block.
func (s *Service) signNewSkipBlock(latest, newest *SkipBlock) (*SkipBlock, *SkipBlock, error) {
	log.Lvl4("Signing new block", newest, "on block", latest)
	if newest != nil && newest.Roster == nil {
		log.Lvl3("Got a data-block")
		if newest.ParentBlockID.IsNull() {
			return nil, nil, errors.New("Data skipblock without parent")
		}
		parent, ok := s.getSkipBlockByID(newest.ParentBlockID)
		if !ok {
			return nil, nil, errors.New("Didn't find parent block")
		}
		newest.Roster = parent.Roster
	}
	// Now verify if it's a valid block
	if err := s.verifyNewSkipBlock(latest, newest); err != nil {
		return nil, nil, errors.New("Verification of newest SkipBlock failed: " + err.Error())
	}

	// Sign it
	err := s.startBFTSignature(newest)
	if err != nil {
		return nil, nil, err
	}
	if err := newest.VerifySignatures(); err != nil {
		log.Error("Couldn't verify signature: " + err.Error())
		return nil, nil, err
	}

	newblocks := make([]*SkipBlock, 1)
	if latest == nil {
		// Genesis-block only
		newblocks[0] = newest
	} else {
		// Adjust forward-links if it's an additional block
		var err error
		newblocks, err = s.addForwardLinks(newest)
		if err != nil {
			return nil, nil, err
		}
		latest = newblocks[1]
	}

	// Store and propagate the new SkipBlocks
	log.Lvl4("Finished signing new block", newest)
	if err = s.startPropagation(newblocks); err != nil {
		return nil, nil, err
	}
	return latest, newblocks[0], nil
}

func (s *Service) startBFTSignature(block *SkipBlock) error {
	done := make(chan bool)
	// create the message we want to sign for this round
	msg := []byte(block.Hash)
	el, err := block.GetResponsible(s)
	if err != nil {
		return err
	}
	switch len(el.List) {
	case 0:
		return errors.New("Found empty Roster")
	case 1:
		return errors.New("Need more than 1 entry for Roster")
	}

	// Start the protocol
	tree := el.GenerateNaryTreeWithRoot(2, s.ServerIdentity())

	node, err := s.CreateProtocolService(skipchainBFT, tree)
	if err != nil {
		return errors.New("Couldn't create new node: " + err.Error())
	}

	// Register the function generating the protocol instance
	root := node.(*bftcosi.ProtocolBFTCoSi)
	root.Msg = msg
	data, err := network.MarshalRegisteredType(block)
	if err != nil {
		return errors.New("Couldn't marshal block: " + err.Error())
	}
	root.Data = data

	// in testing-mode with more than one host and service per cothority-instance
	// we might have the wrong verification-function, so set it again here.
	root.VerificationFunction = s.bftVerify
	// function that will be called when protocol is finished by the root
	root.RegisterOnDone(func() {
		done <- true
	})
	go node.Start()
	select {
	case <-done:
		block.BlockSig = root.Signature()
		if len(block.BlockSig.Exceptions) != 0 {
			return errors.New("Not everybody signed off the new block")
		}
		if err := block.BlockSig.Verify(network.Suite, el.Publics()); err != nil {
			return errors.New("Couldn't verify signature")
		}
	case <-time.After(time.Second * 60):
		return errors.New("Timed out while waiting for signature")
	}
	return nil
}

func (s *Service) verifyNewSkipBlock(latest, newest *SkipBlock) error {
	// Do some sanity-checks on the latest and newest skipblock
	if latest != nil {
		if len(latest.ForwardLink) != 0 {
			return errors.New("Latest already has forward link")
		}
		if !bytes.Equal(newest.BackLinkIds[0], latest.Hash) {
			return errors.New("Newest doesn't point to latest")
		}
	}

	// TODO: add a registration service for verifiers
	return nil
}

// addForwardLinks checks if we have a valid link connecting the two
// SkipBlocks with each other.
func (s *Service) addForwardLinks(newest *SkipBlock) ([]*SkipBlock, error) {
	height := len(newest.BackLinkIds)
	blocks := make([]*SkipBlock, height+1)
	blocks[0] = newest
	for h := range newest.BackLinkIds {
		log.Lvl4("Searching forward-link for", h)
		b, ok := s.getSkipBlockByID(newest.BackLinkIds[h])
		if !ok {
			return nil, errors.New("Found unknwon backlink in block")
		}
		bc := b.Copy()
		log.Lvl4("Checking", b.Index, b, len(bc.ForwardLink))
		if len(bc.ForwardLink) >= h+1 {
			return nil, errors.New("Backlinking to a block which has a forwardlink")
		}
		for len(bc.ForwardLink) < h+1 {
			fl := NewBlockLink()
			fl.Hash = newest.Hash
			bc.ForwardLink = append(bc.ForwardLink, fl)
		}
		log.Lvl4("Block has now height of", len(bc.ForwardLink))
		blocks[h+1] = bc
	}
	return blocks, nil
}

// notify other services about new/updated skipblock
func (s *Service) startPropagation(blocks []*SkipBlock) error {
	log.Lvlf3("Starting to propagate for service %x", s.Context.ServerIdentity().ID[0:8])
	for _, block := range blocks {
		roster := block.Roster
		if roster == nil {
			// Suppose it's a dataSkipBlock
			sb, ok := s.getSkipBlockByID(block.ParentBlockID)
			if !ok {
				return errors.New("Didn't find Roster nor parent")
			}
			roster = sb.Roster
		}
		replies, err := manage.PropagateStartAndWait(s.Context, roster,
			block, propagateTimeout, s.PropagateSkipBlock)
		if err != nil {
			return err
		}
		if replies != len(roster.List) {
			log.Warn("Did only get", replies, "out of", len(roster.List))
		}
	}
	return nil
}

// bftVerify takes a message and verifies it's valid
func (s *Service) bftVerify(msg []byte, data []byte) bool {
	log.Lvlf4("%s verifying block %x", s.ServerIdentity(), msg)
	s.testVerify = true
	_, sbN, err := network.UnmarshalRegistered(data)
	if err != nil {
		log.Error("Couldn't unmarshal SkipBlock", data)
		return false
	}
	sb := sbN.(*SkipBlock)
	if !sb.Hash.Equal(SkipBlockID(msg)) {
		log.Lvlf2("Data skipBlock different from msg %x %x", msg, sb.Hash)
		return false
	}

	f, ok := s.verifiers[sb.VerifierID]
	if !ok {
		log.Lvlf2("Found no user verification for %x", sb.VerifierID)
		return false
	}
	return f(msg, sb)
}

// getSkipBlockByID returns the skip-block or false if it doesn't exist
func (s *Service) getSkipBlockByID(sbID SkipBlockID) (*SkipBlock, bool) {
	s.gMutex.Lock()
	b, ok := s.SkipBlocks[string(sbID)]
	s.gMutex.Unlock()
	return b, ok
}

// storeSkipBlock stores the given SkipBlock in the service-list
func (s *Service) storeSkipBlock(sb *SkipBlock) SkipBlockID {
	s.gMutex.Lock()
	s.SkipBlocks[string(sb.Hash)] = sb
	s.gMutex.Unlock()
	return sb.Hash
}

// lenSkipBlock returns the actual length using mutexes
func (s *Service) lenSkipBlocks() int {
	s.gMutex.Lock()
	defer s.gMutex.Unlock()
	return len(s.SkipBlocks)
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3("Saving service")
	b, err := network.MarshalRegisteredType(s.SkipBlockMap)
	if err != nil {
		log.Error("Couldn't marshal service:", err)
	} else {
		err = ioutil.WriteFile(s.path+"/skipchain.bin", b, 0660)
		if err != nil {
			log.Error("Couldn't save file:", err)
		}
	}
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	configFile := s.path + "/skipchain.bin"
	b, err := ioutil.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Error while reading %s: %s", configFile, err)
	}
	if len(b) > 0 {
		_, msg, err := network.UnmarshalRegistered(b)
		if err != nil {
			return fmt.Errorf("Couldn't unmarshal: %s", err)
		}
		log.Lvl3("Successfully loaded")
		s.SkipBlockMap = msg.(*SkipBlockMap)
	}
	return nil
}

func newSkipchainService(c *sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
		SkipBlockMap:     &SkipBlockMap{make(map[string]*SkipBlock)},
		verifiers:        map[VerifierID]SkipBlockVerifier{},
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	for _, msg := range []interface{}{s.ProposeSkipBlock, s.SetChildrenSkipBlock,
		s.GetUpdateChain} {
		if err := s.RegisterMessage(msg); err != nil {
			log.Fatal("Registration error for msg", msg, err)
		}
	}
	if err := s.RegisterVerification(VerifyShard, s.VerifyShardFunc); err != nil {
		log.Panic(err)
	}
	if err := s.RegisterVerification(VerifyNone, s.VerifyNoneFunc); err != nil {
		log.Panic(err)
	}
	return s
}
