package skipchain

import (
	"crypto/rand"
	"errors"

	"strconv"

	"time"

	"fmt"

	"github.com/dedis/cothority/bftcosi"
	"github.com/dedis/cothority/messaging"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "Skipchain"
const bftForward = "SkipchainBFTForward"

func init() {
	skipchainSID, _ = onet.RegisterNewService(ServiceName, newSkipchainService)
	network.RegisterMessage(&SkipBlockMap{})
}

// Only used in tests
var skipchainSID onet.ServiceID

// Name used to store skipblocks
const skipblocksID = "skipblocks"

// Service handles adding new SkipBlocks
type Service struct {
	*onet.ServiceProcessor
	// SkipBlocks points from SkipBlockID to SkipBlock but SkipBlockID is not a valid
	// key-type for maps, so we need to cast it to string
	*SkipBlockMap
	Propagate messaging.PropagationFunc
	verifiers map[VerifierID]SkipBlockVerifier

	// testVerify is set to true if a verification happened - only for testing
	// TODO: remove
	testVerify bool
}

// ProposeSkipBlock takes a hash for the latest valid SkipBlock and a SkipBlock
// that will be verified. If the verification returns true, the new SkipBlock
// will be signed and added to the chain and returned.
// If the the latest block given is nil it verify if we are actually creating
// the first (genesis) block and creates it. If it is called with nil although
// there already exist previous blocks, it will return an error.
func (s *Service) ProposeSkipBlock(psbd *ProposeSkipBlock) (network.Message, onet.ClientError) {
	prop := psbd.Proposed
	var prev *SkipBlock

	if psbd.LatestID.IsNull() {
		// A new chain is created, suppose all arguments in SkipBlock
		// are correctly set up
		prop.Index = 0
		if prop.MaximumHeight == 0 {
			return nil, onet.NewClientErrorCode(ErrorParameterWrong, "Set a maximumHeight > 0")
		}
		if prop.BaseHeight == 0 {
			return nil, onet.NewClientErrorCode(ErrorParameterWrong, "Set a baseHeight > 0")
		}
		prop.Height = prop.MaximumHeight
		prop.ForwardLink = make([]*BlockLink, 0)
		// genesis block has a random back-link:
		bl := make([]byte, 32)
		rand.Read(bl)
		prop.BackLinkIds = []SkipBlockID{SkipBlockID(bl)}
		prop.GenesisID = prop.updateHash()
	} else {
		// We're appending a block to an existing chain
		var ok bool
		prev, ok = s.GetSkipBlockByID(psbd.LatestID)
		if !ok {
			return nil, onet.NewClientErrorCode(ErrorBlockNotFound, "Didn't find latest block")
		}
		prop.MaximumHeight = prev.MaximumHeight
		prop.BaseHeight = prev.BaseHeight
		prop.ParentBlockID = prev.ParentBlockID
		prop.VerifierID = prev.VerifierID
		prop.Index = prev.Index + 1
		prop.GenesisID = prev.GenesisID
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
				pointer, ok = s.GetSkipBlockByID(pointer.BackLinkIds[0])
				if !ok {
					return nil, onet.NewClientErrorCode(ErrorBlockNotFound, "Didn't find convenient SkipBlock for height "+
						strconv.Itoa(h))
				}
			}
			prop.BackLinkIds[h] = pointer.Hash
		}
		prop.updateHash()
	}
	el, err := prop.GetResponsible(s.SkipBlockMap)
	if err != nil {
		return nil, onet.NewClientErrorCode(ErrorBlockContent, err.Error())
	}
	prop.RespPublic = el.Roster.Publics()

	prop, err = s.updateForwardLinks(prop)
	if err != nil {
		return nil, onet.NewClientErrorCode(ErrorVerification, "Verification error: "+err.Error())
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
func (s *Service) GetUpdateChain(latestKnown *GetUpdateChain) (network.Message, onet.ClientError) {
	block, ok := s.GetSkipBlockByID(latestKnown.LatestID)
	if !ok {
		return nil, onet.NewClientErrorCode(ErrorBlockNotFound, "Couldn't find latest skipblock")
	}
	// at least the latest know and the next block:
	blocks := []*SkipBlock{block}
	log.Lvl3("Starting to search chain")
	for len(block.ForwardLink) > 0 {
		link := block.ForwardLink[len(block.ForwardLink)-1]
		block, ok = s.GetSkipBlockByID(link.Hash)
		if !ok {
			return nil, onet.NewClientErrorCode(ErrorBlockNotFound, "Missing block in forward-chain")
		}
		blocks = append(blocks, block)
	}
	log.Lvl3("Found", len(blocks), "blocks")
	reply := &GetUpdateChainReply{blocks}

	return reply, nil
}

// SetChildrenSkipBlock creates a new SkipChain if that 'service' doesn't exist
// yet.
func (s *Service) SetChildrenSkipBlock(scsb *SetChildrenSkipBlock) (network.Message, onet.ClientError) {
	parentID := scsb.ParentID
	childID := scsb.ChildID
	parent, ok := s.GetSkipBlockByID(parentID)
	if !ok {
		return nil, onet.NewClientErrorCode(ErrorBlockNotFound, "couldn't find skipblock")
	}
	child, ok := s.GetSkipBlockByID(childID)
	if !ok {
		return nil, onet.NewClientErrorCode(ErrorBlockNotFound, "couldn't find skipblock")
	}
	child.ParentBlockID = parentID
	parent.ChildSL = append(parent.ChildSL, childID)

	err := s.startPropagation([]*SkipBlock{child, parent})
	if err != nil {
		return nil, onet.NewClientErrorCode(ErrorVerification, err.Error())
	}

	reply := &SetChildrenSkipBlockReply{parent, child}
	s.save()

	return reply, nil
}

// PropagateSkipBlock will save a new SkipBlock
func (s *Service) PropagateSkipBlock(msg network.Message) {
	sb, ok := msg.(*SkipBlock)
	if !ok {
		log.Error("Couldn't convert to SkipBlock")
		return
	}
	if err := sb.VerifyForwardSignatures(); err != nil {
		log.Error(err)
		return
	}
	s.StoreSkipBlock(sb)
	log.Lvlf3("Stored skip block %+v in %x", *sb, s.Context.ServerIdentity().ID[0:8])
}

// VerifyShardFunc makes sure that the cothority of the child-skipchain is
// part of the root-cothority.
func (s *Service) VerifyShardFunc(msg []byte, sb *SkipBlock) bool {
	if sb.ParentBlockID.IsNull() {
		log.Lvl3("No parent skipblock to verify against")
		return false
	}
	sbParent, exists := s.GetSkipBlockByID(sb.ParentBlockID)
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

// updateForwardLinks asks the roster of every block in the backward-
// links to add a new forward-link.
func (s *Service) updateForwardLinks(proposed *SkipBlock) (accepted *SkipBlock, err error) {
	if err = proposed.VerifyForwardSignatures(); err != nil {
		return
	}
	var newblocks []*SkipBlock
	if proposed.Index > 0 {
		for heightSub, back := range proposed.BackLinkIds {
			height := heightSub + 1
			log.Lvl4("Signing new block", proposed, "on block", back)
			backSB, ok := s.GetSkipBlockByID(back)
			if !ok {
				return nil, errors.New("didn't find block in backlink")
			}

			// Verify that there isn't already a forward-link at the given
			// height.
			if len(backSB.ForwardLink) > height {
				return nil, fmt.Errorf("latest already has forward link at height %d", height)
			}

			// Sign it
			err = s.addForwardLink(backSB, proposed, height)
			if err != nil {
				return
			}
			newblocks = append(newblocks, backSB)

		}
	}
	newblocks = append(newblocks, proposed)
	// Store and propagate the new SkipBlocks
	log.Lvl4("Finished signing new block", proposed, len(newblocks))
	if err = s.startPropagation(newblocks); err != nil {
		return
	}
	accepted = proposed
	return
}

// addForwardLink
func (s *Service) addForwardLink(src, dst *SkipBlock, height int) error {
	if height <= len(src.ForwardLink) {
		return errors.New("already have forward-link at this height")
	}
	// create the message we want to sign for this round
	msg := []byte(dst.Hash)
	resp, err := src.GetResponsible(s.SkipBlockMap)
	if err != nil {
		return err
	}
	roster := resp.Roster
	switch len(roster.List) {
	case 0:
		return errors.New("Found empty Roster")
	case 1:
		return errors.New("Need more than 1 entry for Roster")
	}

	// Start the protocol
	tree := roster.GenerateNaryTreeWithRoot(2, s.ServerIdentity())
	node, err := s.CreateProtocol(bftForward, tree)
	if err != nil {
		return fmt.Errorf("Couldn't create new node: %s", err.Error())
	}

	// Register the function generating the protocol instance
	root := node.(*bftcosi.ProtocolBFTCoSi)
	root.Msg = msg
	data, err := network.Marshal(dst)
	if err != nil {
		return fmt.Errorf("Couldn't marshal block: %s", err.Error())
	}
	root.Data = append(src.Hash, data...)

	// in testing-mode with more than one host and service per cothority-instance
	// we might have the wrong verification-function, so set it again here.
	root.VerificationFunction = s.verifyForwardLink
	// function that will be called when protocol is finished by the root
	done := make(chan bool)
	root.RegisterOnDone(func() {
		done <- true
	})
	go node.Start()
	select {
	case <-done:
		sig := root.Signature()
		if sig.Sig == nil {
			return errors.New("Couldn't sign forward-link")
		}
		fwd := &BlockLink{
			Hash:      dst.Hash,
			Signature: sig.Sig,
		}
		src.ForwardLink = append(src.ForwardLink, fwd)
	case <-time.After(time.Second * 60):
		return errors.New("Timed out while waiting for signature")
	}
	if err = src.VerifyForwardSignatures(); err != nil {
		return errors.New("Wrong BFT-signature: " + err.Error())
	}
	return nil
}

// notify other services about new/updated skipblock
func (s *Service) startPropagation(blocks []*SkipBlock) error {
	log.Lvlf3("Starting to propagate for service %x", s.Context.ServerIdentity().ID[0:8])
	for _, block := range blocks {
		resp, err := block.GetResponsible(s.SkipBlockMap)
		if err != nil {
			return err
		}
		roster := resp.Roster
		replies, err := s.Propagate(roster, block, propagateTimeout)
		if err != nil {
			return err
		}
		if replies != len(roster.List) {
			log.Warn("Did only get", replies, "out of", len(roster.List))
		}
	}
	return nil
}

// verifyForwardLink makes sure that a signature-request for a forward-link
// is valid.
func (s *Service) verifyForwardLink(msg []byte, data []byte) bool {
	log.Lvlf4("%s verifying block %x", s.ServerIdentity(), msg)
	s.testVerify = true
	src := data[0:32]
	srcSB, ok := s.GetSkipBlockByID(src)
	if !ok {
		log.Error("Didn't find src-skipblock")
		return false
	}
	_, dstN, err := network.Unmarshal(data[32:])
	if err != nil {
		log.Error("Couldn't unmarshal SkipBlock", data)
		return false
	}
	dst := dstN.(*SkipBlock)
	if !dst.Hash.Equal(SkipBlockID(msg)) {
		log.Lvlf2("Data skipBlock different from msg %x %x", msg, []byte(dst.Hash))
		return false
	}

	isFree := func() bool {
		for i, b := range dst.BackLinkIds {
			if b.Equal(src) {
				return len(srcSB.ForwardLink) == i
			}
		}
		return false
	}()
	if !isFree {
		log.Lvl2("Didn't find a free corresponding forward-link")
		return false
	}

	f, ok := s.verifiers[dst.VerifierID]
	if !ok {
		log.Lvlf2("Found no user verification for %x", dst.VerifierID)
		return false
	}
	return f(msg, dst)
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3("Saving service")
	err := s.Save(skipblocksID, s.SkipBlockMap)
	if err != nil {
		log.Error("Couldn't save file:", err)
	}
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	if !s.DataAvailable(skipblocksID) {
		return nil
	}
	msg, err := s.Load(skipblocksID)
	if err != nil {
		return err
	}
	var ok bool
	s.SkipBlockMap, ok = msg.(*SkipBlockMap)
	if !ok {
		return errors.New("Data of wrong type")
	}
	return nil
}

func newSkipchainService(c *onet.Context) onet.Service {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		SkipBlockMap:     NewSkipBlockMap(),
		verifiers:        map[VerifierID]SkipBlockVerifier{},
	}
	var err error
	s.Propagate, err = messaging.NewPropagationFunc(c, "SkipchainPropagate", s.PropagateSkipBlock)
	log.ErrFatal(err)
	c.ProtocolRegister(bftForward, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, s.verifyForwardLink)
	})
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	log.ErrFatal(s.RegisterHandlers(s.ProposeSkipBlock, s.SetChildrenSkipBlock,
		s.GetUpdateChain))
	if err := s.RegisterVerification(VerifyShard, s.VerifyShardFunc); err != nil {
		log.Panic(err)
	}
	if err := s.RegisterVerification(VerifyNone, s.VerifyNoneFunc); err != nil {
		log.Panic(err)
	}
	return s
}
