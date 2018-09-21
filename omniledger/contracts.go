package omniledger

import (
	"github.com/dedis/cothority/byzcoin"
)

// ContractConfig ...
// TODO: Complete the function
// The #shard and the epoch-size (in #block) are in the Instruction?
// Add a collection view argument?
// Add view to signature
func (s *Service) ContractConfig(inst byzcoin.Instruction,
	coins []byzcoin.Coin) ([]byzcoin.StateChange, []byzcoin.Coin, error) {

	return nil, nil, nil
}

// NewEpoch ...
// TODO: Complete the function
// The id of the previous block is in the Instruction?
// add view to signature
func (s *Service) NewEpoch(inst byzcoin.Instruction,
	coins []byzcoin.Coin) ([]byzcoin.StateChange, []byzcoin.Coin, error) {

	return nil, nil, nil
}
