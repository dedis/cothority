package skipchain

import (
	"encoding/hex"
	"errors"
	"strings"
	"sync"

	"github.com/dedis/onet/log"
)

// SkipBlockBunch holds all blocks necessary to track this chain up to the
// genesis-block. It can be used by clients to hold all necessary blocks and
// use it as verification for unknown blocks or to update.
type SkipBlockBunch struct {
	GenesisID  SkipBlockID
	Latest     *SkipBlock
	SkipBlocks map[string]*SkipBlock
	Parents    map[string]*SkipBlockBunch
	sync.Mutex
}

// NewSkipBlockBunch returns a pre-initialised SkipBlockBunch. It takes
// a skipblock as an argument, which doesn't have to be the genesis-skipblock.
func NewSkipBlockBunch(sb *SkipBlock) *SkipBlockBunch {
	return &SkipBlockBunch{
		GenesisID:  sb.SkipChainID(),
		Latest:     sb,
		SkipBlocks: map[string]*SkipBlock{string(sb.Hash): sb},
		Parents:    make(map[string]*SkipBlockBunch),
	}
}

// GetByID returns the skip-block or nil if it doesn't exist
func (sbb *SkipBlockBunch) GetByID(sbID SkipBlockID) *SkipBlock {
	sbb.Lock()
	defer sbb.Unlock()
	return sbb.SkipBlocks[string(sbID)]
}

// Store stores the given SkipBlock in the map of skipblocks. If the block is already
// known, only new forward-links and child-links will be added.
func (sbb *SkipBlockBunch) Store(sb *SkipBlock) SkipBlockID {
	sbb.Lock()
	defer sbb.Unlock()
	if sbOld, exists := sbb.SkipBlocks[string(sb.Hash)]; exists {
		// If this skipblock already exists, only copy forward-links and
		// new children.
		if sb.GetForwardLen() > sbOld.GetForwardLen() {
			sb.fwMutex.Lock()
			for _, fl := range sb.ForwardLink[len(sbOld.ForwardLink):] {
				if err := fl.VerifySignature(sbOld.Roster.Publics()); err != nil {
					log.Error("Got a known block with wrong signature in forward-link")
					return nil
				}
				sbOld.AddForward(fl)
			}
			sb.fwMutex.Unlock()
		}
		if len(sb.ChildSL) > len(sbOld.ChildSL) {
			sbOld.ChildSL = append(sbOld.ChildSL, sb.ChildSL[len(sbOld.ChildSL):]...)
		}
	} else {
		sbb.SkipBlocks[string(sb.Hash)] = sb
		sbb.Latest = sb
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
func (sbb *SkipBlockBunch) GetResponsible(sb *SkipBlock) (*SkipBlock, error) {
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
func (sbb *SkipBlockBunch) VerifyLinks(sb *SkipBlock) error {
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
	if !sbBack.GetForward(0).Hash.Equal(sb.Hash) {
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
func (sbb *SkipBlockBunch) GetFuzzy(id string) *SkipBlock {
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
