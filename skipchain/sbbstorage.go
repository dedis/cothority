package skipchain

import (
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/dedis/cothority/skipchain/libsc"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	network.RegisterMessage(&SBBStorage{})
}

// SBBStorage is a convenience-structure to store multiple skipchains in
// your application. This is used in the skipchain-service, scmgr, but can
// also be used in any application that needs to store more than one
// skipchain.
type SBBStorage struct {
	sync.Mutex
	// Stores a bunch for each skipchain
	Bunches map[string]*libsc.SkipBlockBunch
}

// NewSBBStorage returns a pre-initialized structure.
func NewSBBStorage() *SBBStorage {
	return &SBBStorage{
		Bunches: map[string]*libsc.SkipBlockBunch{},
	}
}

// AddBunch takes a new skipblock sb and adds the corresponding
// bunch to SBBStorage.
func (s *SBBStorage) AddBunch(sb *libsc.SkipBlock) *libsc.SkipBlockBunch {
	s.Lock()
	defer s.Unlock()
	if len(s.Bunches) == 0 {
		s.Bunches = make(map[string]*libsc.SkipBlockBunch)
	}
	if s.Bunches[string(sb.SkipChainID())] != nil {
		return nil
	}
	bunch := libsc.NewSkipBlockBunch(sb)
	s.Bunches[string(sb.SkipChainID())] = bunch
	return bunch
}

// Store is a generic storing method that will put the SkipBlock sb
// either in a new bunch or append it to an existing one.
func (s *SBBStorage) Store(sb *libsc.SkipBlock) {
	s.Lock()
	defer s.Unlock()
	bunch, ok := s.Bunches[string(sb.SkipChainID())]
	if !ok {
		bunch = libsc.NewSkipBlockBunch(sb)
		s.Bunches[string(sb.SkipChainID())] = bunch
	}
	bunch.Store(sb)
}

// GetByID searches all bunches for a given skipblockID.
func (s *SBBStorage) GetByID(id libsc.SkipBlockID) *libsc.SkipBlock {
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
func (s *SBBStorage) GetFuzzy(id string) *libsc.SkipBlock {
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
func (s *SBBStorage) GetReg(idRe string) *libsc.SkipBlock {
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
func (s *SBBStorage) GetFromGenesisByID(genesis, id libsc.SkipBlockID) *libsc.SkipBlock {
	sbc := s.GetBunch(genesis)
	if sbc == nil {
		return nil
	}
	return sbc.GetByID(id)
}

// GetBunch returns the bunch corresponding to the genesis-skipblock-id.
func (s *SBBStorage) GetBunch(genesis libsc.SkipBlockID) *libsc.SkipBlockBunch {
	s.Lock()
	defer s.Unlock()
	return s.Bunches[string(genesis)]
}

// GetLatest returns the latest skipblock of the given genesis-id.
func (s *SBBStorage) GetLatest(genesis libsc.SkipBlockID) *libsc.SkipBlock {
	s.Lock()
	defer s.Unlock()
	b, ok := s.Bunches[string(genesis)]
	if ok {
		return b.Latest
	}
	return nil
}

// Update asks the cothority of a block for an update and stores it.
func (s *SBBStorage) Update(sb *libsc.SkipBlock) error {
	block, cerr := NewClient().GetSingleBlock(sb.Roster, sb.Hash)
	if cerr != nil {
		return cerr
	}
	s.Store(block)
	return nil
}

// VerifyLinks checks forward-links and parent-links
func (s *SBBStorage) VerifyLinks(sb *libsc.SkipBlock) error {
	if !sb.CalculateHash().Equal(sb.Hash) {
		return errors.New("wrong hash of skipblock")
	}
	bunch := s.GetBunch(sb.SkipChainID())
	if bunch == nil {
		return errors.New("don't have this skipblock in a bunch")
	}
	if err := bunch.VerifyLinks(sb); err != nil {
		return err
	}
	return nil
}
