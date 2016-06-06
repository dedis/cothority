package skipchain

import (
	"crypto/rand"
	"errors"

	"bytes"

	"sync"

	"strconv"

	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/bftcosi"
	"github.com/dedis/cothority/protocols/manage"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "Skipchain"

func init() {
	sda.RegisterNewService(ServiceName, newSkipchainService)
	skipchainSID = sda.ServiceFactory.ServiceID(ServiceName)
}

var skipchainSID sda.ServiceID

// Service handles adding new SkipBlocks
type Service struct {
	*sda.ServiceProcessor
	// SkipBlocks points from SkipBlockID to SkipBlock but SkipBlockID is not a valid
	// key-type for maps, so we need to cast it to string
	SkipBlocks map[string]*SkipBlock
	sbMutex    sync.Mutex
	path       string
}

// ProposeSkipBlock takes a hash for the latest valid SkipBlock and a SkipBlock
// that will be verified. If the verification returns true, the new SkipBlock
// will be signed and added to the chain and returned.
// If the the latest block given is nil it verify if we are actually creating
// the first (genesis) block and creates it. If it is called with nil although
// there already exist previous blocks, it will return an error.
func (s *Service) ProposeSkipBlock(e *network.Entity, psbd *ProposeSkipBlock) (network.ProtocolMessage, error) {
	prop := psbd.Proposed
	var prev *SkipBlock

	// TODO: support heights > 1

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
		dbg.Lvl4("Found height", prop.Height, "for index", prop.Index,
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
	if prop.EntityList != nil {
		prop.Aggregate = prop.EntityList.Aggregate
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
func (s *Service) GetUpdateChain(e *network.Entity, latestKnown *GetUpdateChain) (network.ProtocolMessage, error) {
	block, ok := s.getSkipBlockByID(latestKnown.LatestID)
	if !ok {
		return nil, errors.New("Couldn't find latest skipblock")
	}
	// at least the latest know and the next block:
	blocks := []*SkipBlock{block}
	dbg.Lvl3("Starting to search chain")
	for len(block.ForwardLink) > 0 {
		link := block.ForwardLink[len(block.ForwardLink)-1]
		block, ok = s.getSkipBlockByID(link.Hash)
		if !ok {
			return nil, errors.New("Missing block in forward-chain")
		}
		blocks = append(blocks, block)
	}
	dbg.Lvl3("Found", len(blocks), "blocks")
	reply := &GetUpdateChainReply{blocks}

	return reply, nil
}

// SetChildrenSkipBlock creates a new SkipChain if that 'service' doesn't exist
// yet.
func (s *Service) SetChildrenSkipBlock(e *network.Entity, scsb *SetChildrenSkipBlock) (network.ProtocolMessage, error) {
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

	return reply, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dbg.Lvl3(s.Entity(), "SkipChain received New Protocol event", tn.ProtocolName(), tn, conf)
	switch tn.ProtocolName() {
	case "Propagate":
		pi, err := manage.NewPropagateProtocol(tn)
		if err != nil {
			return nil, err
		}
		pi.(*manage.Propagate).RegisterOnData(s.PropagateSkipBlock)
		return pi, err
	}
	return nil, nil
}

// PropagateSkipBlock will save a new SkipBlock
func (s *Service) PropagateSkipBlock(msg network.ProtocolMessage) {
	sb, ok := msg.(*SkipBlock)
	if !ok {
		dbg.Error("Couldn't convert to SkipBlock")
		return
	}
	if err := sb.VerifySignatures(); err != nil {
		dbg.Error(err)
		return
	}
	s.storeSkipBlock(sb)
	dbg.Lvlf3("Stored skip block %+v in %x", *sb, s.Context.Entity().ID[0:8])
}

// signNewSkipBlock should start a BFT-signature on the newest block
// which will propagate and update all forward-links of all blocks.
// As a simple solution it verifies the validity of the block,
// simulates a signature and propagates the latest and newest block.
func (s *Service) signNewSkipBlock(latest, newest *SkipBlock) (*SkipBlock, *SkipBlock, error) {
	dbg.Lvl4("Signing new block", newest, "on block", latest)
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
		dbg.Error("Couldn't verify signature: " + err.Error())
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
	dbg.Lvl4("Finished signing new block", newest)
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
	if len(el.List) == 0 {
		return errors.New("Found empty EntityList")
	}

	// Start the protocol
	tree := el.GenerateNaryTreeWithRoot(2, s.Entity())
	node, err := s.CreateProtocolSDA(tree, skipchainBFT)
	if err != nil {
		return errors.New("Couldn't create new node: " + err.Error())
	}

	// Register the function generating the protocol instance
	root := node.(*bftcosi.ProtocolBFTCoSi)
	root.Msg = msg
	// function that will be called when protocol is finished by the root
	root.RegisterOnDone(func() {
		done <- true
	})
	go node.Start()
	select {
	case <-done:
		block.BlockSig = root.Signature()
		if err := block.BlockSig.Verify(network.Suite, el.Aggregate, msg); err != nil {
			return errors.New("Couldn't verify signature")
		}
		return nil
	case <-time.After(time.Second * 3):
		return errors.New("Timed out while waiting for signature")
	}
	return errors.New("Nothing happened...")
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
		dbg.Lvl4("Searching forward-link for", h)
		b, ok := s.getSkipBlockByID(newest.BackLinkIds[h])
		if !ok {
			return nil, errors.New("Found unknwon backlink in block")
		}
		bc := b.Copy()
		dbg.Lvl4("Checking", b.Index, b, len(bc.ForwardLink))
		if len(bc.ForwardLink) >= h+1 {
			return nil, errors.New("Backlinking to a block which has a forwardlink")
		}
		for len(bc.ForwardLink) < h+1 {
			fl := NewBlockLink()
			fl.Hash = newest.Hash
			bc.ForwardLink = append(bc.ForwardLink, fl)
		}
		dbg.Lvl4("Block has now height of", len(bc.ForwardLink))
		blocks[h+1] = bc
	}
	return blocks, nil
}

// notify other services about new/updated skipblock
func (s *Service) startPropagation(blocks []*SkipBlock) error {
	dbg.Lvlf3("Starting to propagate for service %x", s.Context.Entity().ID[0:8])
	for _, block := range blocks {
		roster := block.EntityList
		if roster == nil {
			// Suppose it's a dataSkipBlock
			sb, ok := s.getSkipBlockByID(block.ParentBlockID)
			if !ok {
				return errors.New("Didn't find EntityList nor parent")
			}
			roster = sb.EntityList
		}
		replies, err := manage.PropagateStartAndWait(s, roster,
			block, 1000, s.PropagateSkipBlock)
		if err != nil {
			return err
		}
		if replies != len(roster.List) {
			dbg.Warn("Did only get", replies, "out of", len(roster.List))
		}
	}
	return nil
}

// bftVerify takes a message and verifies it's valid
func (s *Service) bftVerify(msg []byte) bool {
	return true
}

// getSkipBlockByID returns the skip-block or false if it doesn't exist
func (s *Service) getSkipBlockByID(sbID SkipBlockID) (*SkipBlock, bool) {
	s.sbMutex.Lock()
	b, ok := s.SkipBlocks[string(sbID)]
	s.sbMutex.Unlock()
	return b, ok
}

// storeSkipBlock stores the given SkipBlock in the service-list
func (s *Service) storeSkipBlock(sb *SkipBlock) SkipBlockID {
	s.sbMutex.Lock()
	s.SkipBlocks[string(sb.Hash)] = sb
	s.sbMutex.Unlock()
	return sb.Hash
}

// lenSkipBlock returns the actual length using mutexes
func (s *Service) lenSkipBlocks() int {
	s.sbMutex.Lock()
	defer s.sbMutex.Unlock()
	return len(s.SkipBlocks)
}

const skipchainBFT = "SkipchainBFT"

func newSkipchainService(c sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
		SkipBlocks:       make(map[string]*SkipBlock),
	}
	sda.ProtocolRegisterName(skipchainBFT, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, s.bftVerify)
	})
	if err := s.RegisterMessage(s.ProposeSkipBlock); err != nil {
		dbg.Fatal("Registration error:", err)
	}
	if err := s.RegisterMessage(s.SetChildrenSkipBlock); err != nil {
		dbg.Fatal("Registration error:", err)
	}
	if err := s.RegisterMessage(s.GetUpdateChain); err != nil {
		dbg.Fatal("Registration error:", err)
	}
	return s
}
