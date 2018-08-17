package service

import (
	"errors"

	"github.com/dedis/cothority/omniledger/darc"
	ol "github.com/dedis/cothority/omniledger/service"
)

// This file holds the contracts for the pop-party. The following contracts
// are defined here:
//   - PopParty - holds the Configuration and later the FinalStatement
//   - PopCoinAccount - represents an account of popcoins

// ContractPopParty represents a pop-party that holds either a configuration
// or a final statement.
var ContractPopParty = "popParty"

// ContractPopCoinAccount holds popcoins of an attendee or a service.
var ContractPopCoinAccount = "popCoinAccount"

func (s *Service) ContractPopParty(cdb ol.CollectionView, inst ol.Instruction, coins []ol.Coin) (sc []ol.StateChange, cOut []ol.Coin, err error) {
	cOut = coins
	switch {
	case inst.Spawn != nil:
		cfg := inst.Spawn.Args.Search("PartyConfig")
		return ol.StateChanges{
			ol.NewStateChange(ol.Create, inst.DeriveID("config"), inst.Spawn.ContractID, cfg, darc.ID(inst.InstanceID[:])),
		}, cOut, nil
	case inst.Invoke != nil:
		// switch inst.Invoke.Command {
		// case "evolve":
		// 	var darcID darc.ID
		// 	_, _, darcID, err = cdb.GetValues(inst.InstanceID.Slice())
		// 	if err != nil {
		// 		return
		// 	}
		//
		// 	darcBuf := inst.Invoke.Args.Search("darc")
		// 	newD, err := darc.NewFromProtobuf(darcBuf)
		// 	if err != nil {
		// 		return nil, nil, err
		// 	}
		// 	oldD, err := LoadDarcFromColl(cdb, darcID)
		// 	if err != nil {
		// 		return nil, nil, err
		// 	}
		// 	if err := newD.SanityCheck(oldD); err != nil {
		// 		return nil, nil, err
		// 	}
		// 	return []StateChange{
		// 		NewStateChange(Update, inst.InstanceID, ContractDarcID, darcBuf, darcID),
		// 	}, coins, nil
		// default:
		return nil, nil, errors.New("invalid command: " + inst.Invoke.Command)
		// }
	default:
		return nil, nil, errors.New("Only invoke and spawn are defined yet")
	}
}

func (s *Service) ContractPopCoinAccount(cdb ol.CollectionView, inst ol.Instruction, coins []ol.Coin) (sc []ol.StateChange, c []ol.Coin, err error) {
	return nil, coins, nil
}
