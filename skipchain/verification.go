package skipchain

import "go.dedis.ch/onet/v3/log"

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

	prev := s.db.GetByID(newSB.BackLinkIDs[0])
	if prev == nil {
		return false
	}

	if !prev.SkipChainID().Equal(newSB.SkipChainID()) {
		return false
	}
	if prev.MaximumHeight != newSB.MaximumHeight {
		return false
	}
	if prev.BaseHeight != newSB.BaseHeight {
		return false
	}
	if prev.Index+1 != newSB.Index {
		return false
	}
	if prev.SignatureScheme > newSB.SignatureScheme {
		// the signature scheme can only have an index higher than the previous blocks
		// so that no one can downgrade the verification
		return false
	}

	log.Lvl4("No verification - accepted")
	return true
}
