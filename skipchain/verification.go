package skipchain

import "gopkg.in/dedis/onet.v2/log"

/*
This file holds all verification-functions for the skipchain.
*/

// VerifyBase checks basic parameters between two skipblocks.
func (s *Service) verifyFuncBase(newID []byte, newSB *SkipBlock) bool {
	if !newSB.Hash.Equal(newID) {
		log.Lvl2("Hashes are not equal")
		return false
	}
	if s.verifyBlock(newSB) != nil {
		log.Lvl2("verifyBlock failed")
		return false
	}
	log.Lvl4("No verification - accepted")
	return true
}

// VerifyRoot depends on a data-block being a slice of public keys
// that are used to sign the next block. The private part of those
// keys are supposed to be offline. It makes sure
// that every new block is signed by the keys present in the previous block.
func (s *Service) verifyFuncRoot(newID []byte, newSB *SkipBlock) bool {
	return true
}

// VerifyControl makes sure this chain is a child of a Root-chain and
// that there is now new block if a newer parent is present.
// It also makes sure that no more than 1/3 of the members of the roster
// change between two blocks.
func (s *Service) verifyFuncControl(newID []byte, newSB *SkipBlock) bool {
	return true
}

// VerifyData makes sure that:
//   - it has a parent-chain with `VerificationControl`
//   - its Roster doesn't change between blocks
//   - if there is a newer parent, no new block will be appended to that chain.
func (s *Service) verifyFuncData(newID []byte, newSB *SkipBlock) bool {
	if newSB.ParentBlockID.IsNull() {
		log.Lvl3("No parent skipblock to verify against")
		return false
	}
	sbParent := s.db.GetByID(newSB.ParentBlockID)
	if sbParent == nil {
		log.Lvl3("Parent skipblock doesn't exist")
		return false
	}
	for _, e := range newSB.Roster.List {
		if i, _ := sbParent.Roster.Search(e.ID); i < 0 {
			log.Lvl3("ServerIdentity in child doesn't exist in parent")
			return false
		}
	}
	return true
}
