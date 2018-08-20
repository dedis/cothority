package service

import (
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	ol "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
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
		fsBuf := inst.Spawn.Args.Search("FinalStatement")
		if fsBuf == nil {
			return nil, nil, errors.New("need FinalStatement argument")
		}
		ppData := &PopPartyInstance{
			State:          1,
			FinalStatement: &FinalStatement{},
		}
		err = protobuf.DecodeWithConstructors(fsBuf, ppData.
			FinalStatement, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return nil, nil, errors.New("couldn't unmarshal the final statement: " + err.Error())
		}
		ppiBuf, err := protobuf.Encode(ppData)
		if err != nil {
			return nil, nil, errors.New("couldn't marshal PopPartyInstance: " + err.Error())
		}
		return ol.StateChanges{
			ol.NewStateChange(ol.Create, inst.DeriveID(""), inst.Spawn.ContractID, ppiBuf, darc.ID(inst.InstanceID[:])),
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
