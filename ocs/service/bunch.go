package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

func init() {
	for _, m := range []interface{}{
		// - Data structures
		&SkipBlockBunch{},
		&SBBStorage{},
	} {
		network.RegisterMessage(m)
	}

}

// SkipBlockBunch holds all blocks necessary to track this chain up to the
// genesis-block. It can be used by clients to hold all necessary blocks and
// use it as verification for unknown blocks or to update.
type SkipBlockBunch struct {
	GenesisID  skipchain.SkipBlockID
	Latest     *skipchain.SkipBlock
	SkipBlocks map[string]*skipchain.SkipBlock
	Parents    map[string]*SkipBlockBunch
	sync.Mutex
}

// NewSkipBlockBunch returns a pre-initialised SkipBlockBunch. It takes
// a skipblock as an argument, which doesn't have to be the genesis-skipblock.
func NewSkipBlockBunch(sb *skipchain.SkipBlock) *SkipBlockBunch {
	return &SkipBlockBunch{
		GenesisID:  sb.SkipChainID(),
		Latest:     sb,
		SkipBlocks: map[string]*skipchain.SkipBlock{string(sb.Hash): sb},
		Parents:    make(map[string]*SkipBlockBunch),
	}
}

// GetByID returns the skip-block or nil if it doesn't exist
func (sbb *SkipBlockBunch) GetByID(sbID skipchain.SkipBlockID) *skipchain.SkipBlock {
	sbb.Lock()
	defer sbb.Unlock()
	return sbb.SkipBlocks[string(sbID)]
}

// Store stores the given SkipBlock in the map of skipblocks. If the block is already
// known, only new forward-links and child-links will be added.
func (sbb *SkipBlockBunch) Store(sb *skipchain.SkipBlock) skipchain.SkipBlockID {
	sbb.Lock()
	defer sbb.Unlock()
	if sbOld, exists := sbb.SkipBlocks[string(sb.Hash)]; exists {
		// If this skipblock already exists, only copy forward-links and
		// new children.
		if sb.GetForwardLen() > sbOld.GetForwardLen() {
			for _, fl := range sb.ForwardLink[len(sbOld.ForwardLink):] {
				if err := fl.Verify(cothority.Suite, sbOld.Roster.Publics()); err != nil {
					log.Error("Got a known block with wrong signature in forward-link")
					return nil
				}
				sbOld.AddForward(fl)
			}
		}
		if len(sb.ChildSL) > len(sbOld.ChildSL) {
			sbOld.ChildSL = append(sbOld.ChildSL, sb.ChildSL[len(sbOld.ChildSL):]...)
		}
	} else {
		sbb.SkipBlocks[string(sb.Hash)] = sb
		if sb.Index > sbb.Latest.Index {
			sbb.Latest = sb
		}
	}
	return sb.Hash
}

// Length returns the actual length using mutexes
func (sbb *SkipBlockBunch) Length() int {
	sbb.Lock()
	defer sbb.Unlock()
	return len(sbb.SkipBlocks)
}

// GetResponsible searches for the block that is responsible for sb
// - Root_Genesis - himself
// - *_Gensis - it's his parent
// - else - it's the previous block
func (sbb *SkipBlockBunch) GetResponsible(sb *skipchain.SkipBlock) (*skipchain.SkipBlock, error) {
	if sb == nil {
		log.Panic(log.Stack())
	}
	if sb.Index == 0 {
		// Genesis-block
		if sb.ParentBlockID.IsNull() {
			// Root-skipchain, no other parent
			return sb, nil
		}
		ret := sbb.GetByID(sb.ParentBlockID)
		if ret == nil {
			return nil, errors.New("No Roster and no parent")
		}
		return ret, nil
	}
	if len(sb.BackLinkIDs) == 0 {
		return nil, errors.New("Invalid block: no backlink")
	}
	prev := sbb.GetByID(sb.BackLinkIDs[0])
	if prev == nil {
		return nil, errors.New("Didn't find responsible")
	}
	return prev, nil
}

// VerifyLinks makes sure that all forward- and backward-links are correct.
// It takes a skipblock to verify and returns nil in case of success.
func (sbb *SkipBlockBunch) VerifyLinks(sb *skipchain.SkipBlock) error {
	if len(sb.BackLinkIDs) == 0 {
		return errors.New("need at least one backlink")
	}

	if err := sb.VerifyForwardSignatures(); err != nil {
		return errors.New("Wrong signatures: " + err.Error())
	}

	// Verify if we're in the responsible-list
	if !sb.ParentBlockID.IsNull() {
		parent := sbb.GetByID(sb.ParentBlockID)
		if parent == nil {
			return errors.New("Didn't find parent")
		}
		if err := parent.VerifyForwardSignatures(); err != nil {
			return err
		}
		found := false
		for _, child := range parent.ChildSL {
			if child.Equal(sb.Hash) {
				found = true
				break
			}
		}
		if !found {
			return errors.New("parent doesn't know about us")
		}
	}

	// We don't check backward-links for genesis-blocks
	if sb.Index == 0 {
		return nil
	}

	// Verify we're referenced by our previous block
	sbBack := sbb.GetByID(sb.BackLinkIDs[0])
	if sbBack == nil {
		if sb.GetForwardLen() > 0 {
			log.LLvl3("Didn't find back-link, but have a good forward-link")
			return nil
		}
		return errors.New("Didn't find height-0 skipblock in sbm")
	}
	if err := sbBack.VerifyForwardSignatures(); err != nil {
		return err
	}
	if !sbBack.GetForward(0).Hash().Equal(sb.Hash) {
		return errors.New("didn't find our block in forward-links")
	}
	return nil
}

// GetFuzzy searches for a block that resembles the given ID, if ID is not full.
// If there are multiple matching skipblocks, the first one is chosen. If none
// match, nil will be returned.
//
// The search is done in the following order:
//  1. as prefix
//  2. if none is found - as suffix
//  3. if none is found - anywhere
func (sbb *SkipBlockBunch) GetFuzzy(id string) *skipchain.SkipBlock {
	for _, sb := range sbb.SkipBlocks {
		if strings.HasPrefix(hex.EncodeToString(sb.Hash), id) {
			return sb
		}
	}
	for _, sb := range sbb.SkipBlocks {
		if strings.HasSuffix(hex.EncodeToString(sb.Hash), id) {
			return sb
		}
	}
	for _, sb := range sbb.SkipBlocks {
		if strings.Contains(hex.EncodeToString(sb.Hash), id) {
			return sb
		}
	}
	return nil
}

// SBBStorage is a convenience-structure to store multiple skipchains in
// your application. This is used in the skipchain-service, scmgr, but can
// also be used in any application that needs to store more than one
// skipchain.
type SBBStorage struct {
	sync.Mutex
	// Stores a bunch for each skipchain
	Bunches map[string]*SkipBlockBunch
}

// NewSBBStorage returns a pre-initialized structure.
func NewSBBStorage() *SBBStorage {
	return &SBBStorage{
		Bunches: map[string]*SkipBlockBunch{},
	}
}

// AddBunch takes a new skipblock sb and adds the corresponding
// bunch to SBBStorage.
func (s *SBBStorage) AddBunch(sb *skipchain.SkipBlock) *SkipBlockBunch {
	s.Lock()
	defer s.Unlock()
	if len(s.Bunches) == 0 {
		s.Bunches = make(map[string]*SkipBlockBunch)
	}
	if s.Bunches[string(sb.SkipChainID())] != nil {
		log.Error("That bunch already exists")
		return nil
	}
	bunch := NewSkipBlockBunch(sb)
	s.Bunches[string(sb.SkipChainID())] = bunch
	return bunch
}

// Store is a generic storing method that will put the SkipBlock sb
// either in a new bunch or append it to an existing one.
func (s *SBBStorage) Store(sb *skipchain.SkipBlock) {
	s.Lock()
	defer s.Unlock()
	bunch, ok := s.Bunches[string(sb.SkipChainID())]
	if !ok {
		bunch = NewSkipBlockBunch(sb)
		s.Bunches[string(sb.SkipChainID())] = bunch
	}
	bunch.Store(sb)
}

// GetByID searches all bunches for a given skipblockID.
func (s *SBBStorage) GetByID(id skipchain.SkipBlockID) *skipchain.SkipBlock {
	s.Lock()
	defer s.Unlock()
	for _, b := range s.Bunches {
		if sb := b.GetByID(id); sb != nil {
			return sb
		}
	}
	return nil
}

// GetFuzzy searches all bunches for a given ID and returns the first
// SkipBlock that matches. It searches first all beginnings of SkipBlockIDs,
// then all endings, and finally all in-betweens.
func (s *SBBStorage) GetFuzzy(id string) *skipchain.SkipBlock {
	sb := s.GetReg("^" + id)
	if sb != nil {
		return sb
	}
	sb = s.GetReg(id + "$")
	if sb != nil {
		return sb
	}
	sb = s.GetReg(id)
	if sb != nil {
		return sb
	}

	return nil
}

// GetReg searches for the regular-expression in all skipblock-ids.
func (s *SBBStorage) GetReg(idRe string) *skipchain.SkipBlock {
	re, err := regexp.Compile(idRe)
	if err != nil {
		return nil
	}
	for _, b := range s.Bunches {
		for _, sb := range b.SkipBlocks {
			if re.MatchString(fmt.Sprintf("%x", sb.Hash)) {
				return sb
			}
		}
	}
	return nil
}

// GetFromGenesisByID returns the skipblock directly from a given genesis-
// and block-id. This is faster than GetByID.
func (s *SBBStorage) GetFromGenesisByID(genesis, id skipchain.SkipBlockID) *skipchain.SkipBlock {
	sbc := s.GetBunch(genesis)
	if sbc == nil {
		return nil
	}
	return sbc.GetByID(id)
}

// GetBunch returns the bunch corresponding to the genesis-skipblock-id.
func (s *SBBStorage) GetBunch(genesis skipchain.SkipBlockID) *SkipBlockBunch {
	s.Lock()
	defer s.Unlock()
	return s.Bunches[string(genesis)]
}

// GetLatest returns the latest skipblock of the given genesis-id.
func (s *SBBStorage) GetLatest(genesis skipchain.SkipBlockID) *skipchain.SkipBlock {
	s.Lock()
	defer s.Unlock()
	b, ok := s.Bunches[string(genesis)]
	if ok {
		return b.Latest
	}
	return nil
}
