package byzcoin

import (
	"errors"

	"go.dedis.ch/cothority/v3/skipchain"
)

// GlobalState is used to query for any data in byzcoin.
type GlobalState interface {
	ReadOnlyStateTrie
	ReadOnlySkipChain
	NewStateFromTrie(rst ReadOnlyStateTrie) GlobalState
}

// ReadOnlySkipChain holds the skipchain data.
type ReadOnlySkipChain interface {
	GetLatest() (*skipchain.SkipBlock, error)
	GetGenesisBlock() (*skipchain.SkipBlock, error)
	GetBlock(skipchain.SkipBlockID) (*skipchain.SkipBlock, error)
	GetBlockByIndex(idx int) (*skipchain.SkipBlock, error)
}

type globalState struct {
	ReadOnlyStateTrie
	ReadOnlySkipChain
}

func (gs globalState) NewStateFromTrie(rst ReadOnlyStateTrie) GlobalState {
	return &globalState{
		ReadOnlyStateTrie: rst,
		ReadOnlySkipChain: gs.ReadOnlySkipChain,
	}
}

var _ GlobalState = (*globalState)(nil)

type roSkipChain struct {
	inner      *skipchain.Service
	genesisID  skipchain.SkipBlockID
	currLatest skipchain.SkipBlockID
}

func newROSkipChain(s *skipchain.Service, genesisID skipchain.SkipBlockID) *roSkipChain {
	return &roSkipChain{inner: s, genesisID: genesisID, currLatest: genesisID}
}

func (s *roSkipChain) GetLatest() (*skipchain.SkipBlock, error) {
	sb, err := s.inner.GetDB().GetLatestByID(s.currLatest)
	if err != nil {
		return nil, err
	}
	s.currLatest = sb.CalculateHash()
	return sb, nil
}

func (s *roSkipChain) GetGenesisBlock() (*skipchain.SkipBlock, error) {
	reply, err := s.inner.GetSingleBlockByIndex(
		&skipchain.GetSingleBlockByIndex{
			Genesis: s.genesisID,
			Index:   0,
		})
	if err != nil {
		return nil, err
	}
	return reply.SkipBlock, nil
}

func (s *roSkipChain) GetBlock(id skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
	sb := s.inner.GetDB().GetByID(id)
	if sb == nil {
		return nil, errors.New("block not found")
	}
	return sb, nil
}

func (s *roSkipChain) GetBlockByIndex(idx int) (*skipchain.SkipBlock, error) {
	return nil, errors.New("not implemented")
}
