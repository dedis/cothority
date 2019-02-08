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
	log.Lvl4("No verification - accepted")
	return true
}
