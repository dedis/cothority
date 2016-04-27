package skipchain

import (
	"crypto/rand"
	"errors"

	"fmt"

	"bytes"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

func init() {
	sda.RegisterNewService("Skipchain", newSkipchainService)
	skipchainSID = sda.ServiceFactory.ServiceID("Skipchain")
}

var skipchainSID sda.ServiceID

// Service handles adding new SkipBlocks
type Service struct {
	*sda.ServiceProcessor
	// SkipBlocks points from SkipBlockID to SkipBlock but SkipBlockID is not a valid
	// key-type for maps, so we need to cast it to string
	SkipBlocks map[string]SkipBlock
	path       string
}

// ProposeSkipBlock takes a hash for the latest valid SkipBlock and a SkipBlock
// that will be verified. If the verification returns true, the new SkipBlock
// will be signed and added to the chain and returned.
// If the given nil as the latest block it verify if we are actually creating
// the first (genesis) block and create it. If it is called with nil although
// there already exist previous blocks, it will return an error.
func (s *Service) ProposeSkipBlock(latest SkipBlockID, proposed SkipBlock) (*ProposedSkipBlockReply, error) {
	if latest == nil && len(s.SkipBlocks) == 0 { // genesis block creation
		s.updateSkipBlock(nil, proposed)
		reply := &ProposedSkipBlockReply{
			Previous: nil, // genesis block
			Latest:   proposed,
		}
		dbg.LLvl3(fmt.Sprintf("Successfuly created genesis: %+v", reply))
		return reply, nil
	}

	prev, ok := s.SkipBlocks[string(latest)]
	if !ok {
		return nil, errors.New("Couldn't find latest block.")
	}
	if s.verifyNewSkipBlock(prev, proposed) {
		s.updateSkipBlock(prev, proposed)
		reply := &ProposedSkipBlockReply{
			Previous: prev,
			Latest:   proposed,
		}
		return reply, nil
	}

	return nil, errors.New("Verification of proposed block failed.")
}

func (s *Service) updateSkipBlock(prev, proposed SkipBlock) {
	dbg.LLvl4(fmt.Sprintf("prev=%+v\nproposed=%+v", prev, proposed))
	sbc := proposed.GetCommon()
	var curID string
	if prev == nil { // genesis
		sbc.Index++
		// genesis block has a random back-link:
		sbc.BackLink = make([]SkipBlockID, 1)
		bl := make([]byte, 32)
		_, _ = rand.Read(bl)
		sbc.BackLink[0] = bl
		// empty forward link:
		sbc.ForwardLink = make([]BlockLink, 1)

		curID = string(proposed.updateHash())
	} else {
		prevCommon := prev.GetCommon()
		sbc.Index = prevCommon.Index + 1
		sbc.BackLink = make([]SkipBlockID, 1)
		sbc.BackLink[0] = prev.updateHash()
		// update forward link of previous block:
		curHashID := proposed.updateHash()
		prevCommon.ForwardLink = append(prevCommon.ForwardLink,
			BlockLink{Hash: curHashID,
				Signature: cosi.NewSignature(network.Suite), // FIXME get real signature
			})

		curID = string(curHashID)
	}
	// update
	s.SkipBlocks[curID] = proposed
}

// GetUpdateChain returns a slice of SkipBlocks which describe the path from the
// latest known block the caller knows of to the actual latest SkipBlock.
// Comparable to search in SkipLists.
func (s *Service) GetUpdateChain(latestKnown SkipBlockID) (*GetUpdateChainReply, error) {
	block, ok := s.SkipBlocks[string(latestKnown)]
	if !ok {
		return nil, errors.New("Couldn't find latest skipblock")
	}
	// at least the latest know and the next block:
	path := s.followForward(block)
	reply := &GetUpdateChainReply{
		Update: path,
	}

	return reply, nil
}

func (s *Service) followForward(sb SkipBlock) []SkipBlock {
	path := make([]SkipBlock, 1)
	// add current
	path[0] = sb
	forwardlinks := sb.GetCommon().ForwardLink
	for _, linkId := range forwardlinks {
		if linkId.Hash != nil {
			sb := s.SkipBlocks[string(linkId.Hash)]
			path = append(path, sb)
			path = append(path, s.followForwardInternal(sb)...)
		}
	}
	return path
}

func (s *Service) followForwardInternal(curSb SkipBlock) []SkipBlock {
	path := make([]SkipBlock, 0)
	forwardlinks := curSb.GetCommon().ForwardLink
	for _, linkId := range forwardlinks {
		if linkId.Hash != nil {
			sb := s.SkipBlocks[string(linkId.Hash)]
			path = append(path, sb)
			path = append(path, s.followForwardInternal(sb)...)
		}
	}
	return path
}

// SetChildrenSkipBlock creates a new SkipChain if that 'service' doesn't exist
// yet.
func (s *Service) SetChildrenSkipBlock(parent, child SkipBlockID) error {
	return nil
}

// GetChildrenSkipList creates a new SkipChain if that 'service' doesn't exist
// yet.
func (s *Service) GetChildrenSkipList(sb SkipBlock, verifier VerifierID) (*GetUpdateChainReply, error) {
	return nil, nil
}

// PropagateSkipBlock sends a newly signed SkipBlock to all members of
// the Cothority
func (s *Service) PropagateSkipBlock(latest SkipBlock) {

}

// SignBlock signs off the new block pointed to by the hash by first
// verifying its validity and then collectively signing off the block.
// The new signature is NOT broadcasted to the roster!
func (s *Service) SignBlock(sb SkipBlock) error {
	prev, ok := s.SkipBlocks[string(sb.GetCommon().BackLink[0])]
	if !ok {
		return errors.New("Didn't find SkipBlock")
	}
	if !s.verifyNewSkipBlock(prev, sb) {
		return errors.New("Refused")
	}
	// TODO: sign off the block with the roster
	sb.GetCommon().Signature = cosi.NewSignature(network.Suite)
	return nil
}

// ForwardSignature asks this responsible for a SkipChain to sign off
// a new ForwardLink. Upon success the new signature will be
// broadcast to the entire roster and all backward- and forward-links.
// It returns the SkipBlock with the updated ForwardSignature or an error.
func (s *Service) ForwardSignature(updating *ForwardSignature) (SkipBlock, error) {
	sb, ok := s.SkipBlocks[string(updating.ToUpdate)]
	if !ok {
		return nil, errors.New("Didn't find SkipBlock")
	}
	if updating.Latest.VerifySignatures() != nil {
		return nil, errors.New("Couldn't verify signature of new block")
	}
	comm := sb.GetCommon()
	commLatest := updating.Latest.GetCommon()
	updateHeight := 0
	latestHeight := len(commLatest.BackLink)
	for updateHeight = 0; updateHeight < latestHeight; updateHeight++ {
		if bytes.Equal(commLatest.BackLink[updateHeight], comm.Hash()) {
			break
		}
	}
	if updateHeight == latestHeight {
		return nil, errors.New("Didn't find ourselves in the backlinks")
	}
	currHeight := len(comm.ForwardLink)
	if currHeight == 0 {
		comm.ForwardLink = make([]*BlockLink, 0, comm.Height)
		// As we are the direct predecessor of the block, we need
		// to verify using the verification-function whether that
		// block is valid or not.
		if !s.verifyNewSkipBlock(sb, updating.Latest) {
			return nil, errors.New("New SkipBlock not accepted!")
		}
	} else {
		// We only need to verify that we have a complete link-history
		// from ourselves to the proposed SkipBlock
		if !s.verifyLinkedSkipBlock(sb, updating.Latest) {
			return nil, errors.New("Didn't find a valid update-path")
		}
	}
	comm.ForwardLink[currHeight].Hash = updating.Latest.GetCommon().Hash()

	// TODO: sign off on the forward-link
	return sb, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dbg.Lvl1("SkipChain received New Protocol event", tn, conf)
	return nil, nil
}

// verifyNewSkipBlock calls the appropriate app-verification and returns
// either a signature on the newest SkipBlock or nil if the SkipBlock
// has been refused
func (s *Service) verifyNewSkipBlock(latest, newest SkipBlock) bool {
	// TODO: implement a couple of protocols that can check all
	// TODO: Verify* constants
	return true
}

// verifyLinkedSkipBlock checks if we have a valid link connecting the two
// SkipBlocks with each other.
func (s *Service) verifyLinkedSkipBlock(latest, newest SkipBlock) bool {
	// TODO: check we have a valid link
	return true
}

func newSkipchainService(c sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
		SkipBlocks:       make(map[string]SkipBlock),
	}
	return s
}
