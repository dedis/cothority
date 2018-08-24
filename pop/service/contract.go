package service

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/contracts"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/omniledger/darc/expression"
	ol "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet/log"
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
	var darcID darc.ID
	var ppi PopPartyInstance
	if inst.Spawn == nil {
		var ppiBuf []byte
		ppiBuf, _, darcID, err = cdb.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return nil, nil, errors.New("couldn't get instance data: " + err.Error())
		}
		err = protobuf.DecodeWithConstructors(ppiBuf, &ppi, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return nil, nil, errors.New("couldn't unmarshal existing PopPartyInstance: " + err.Error())
		}
	} else {
		darcID = inst.InstanceID.Slice()
	}
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
		// partyID, err := ppData.FinalStatement.Hash()
		// if err != nil {
		// 	return nil, nil, errors.New("couldn't get party id: " + err.Error())
		// }
		// log.Printf("New party: %x", partyID)
		return ol.StateChanges{
			ol.NewStateChange(ol.Create, inst.DeriveID(""), inst.Spawn.ContractID, ppiBuf, darc.ID(inst.InstanceID[:])),
		}, cOut, nil
	case inst.Invoke != nil:
		switch inst.Invoke.Command {
		case "Finalize":
			if ppi.State != 1 {
				return nil, nil, fmt.Errorf("can only finalize party with state 1, but current state is %d",
					ppi.State)
			}
			fsBuf := inst.Invoke.Args.Search("FinalStatement")
			if fsBuf == nil {
				return nil, nil, errors.New("missing argument: FinalStatement")
			}
			fs := FinalStatement{}
			err = protobuf.DecodeWithConstructors(fsBuf, &fs, network.DefaultConstructors(cothority.Suite))
			if err != nil {
				return nil, nil, errors.New("argument is not a valid FinalStatement")
			}

			// TODO: check for aggregate signature of all organizers

			ppi := PopPartyInstance{
				State:          2,
				FinalStatement: &fs,
			}
			ppiBuf, err := protobuf.Encode(&ppi)
			if err != nil {
				return nil, nil, errors.New("couldn't marshal PopPartyInstance: " + err.Error())
			}

			// Update existing final statement
			scs := ol.StateChanges{
				ol.NewStateChange(ol.Update, inst.InstanceID, ContractPopParty, ppiBuf, darcID),
			}

			for i, pub := range fs.Attendees {
				log.LLvl3("Creating darc for attendee", i)
				id := darc.NewIdentityEd25519(pub)
				rules := darc.InitRules([]darc.Identity{id}, []darc.Identity{id})
				rules.AddRule(darc.Action("invoke:transfer"), expression.Expr(id.String()))
				d := darc.NewDarc(rules, []byte("Attendee darc for pop-party"))
				darcBuf, err := d.ToProto()
				if err != nil {
					return nil, nil, errors.New("couldn't marshal darc: " + err.Error())
				}
				log.LLvlf3("%s: Final %x/%x", s.ServerIdentity(), d.GetBaseID(), sha256.Sum256(darcBuf))
				scs = append(scs, ol.NewStateChange(ol.Create, ol.NewInstanceID(d.GetBaseID()),
					ol.ContractDarcID, darcBuf, darcID))

				log.LLvl3("Creating account for attendee", i)
				iid := sha256.New()
				iid.Write(inst.InstanceID.Slice())
				pubBuf, err := pub.MarshalBinary()
				if err != nil {
					return nil, nil, errors.New("couldn't marshal public key: " + err.Error())
				}
				iid.Write(pubBuf)
				cci := contracts.CoinInstance{
					Type:    []byte("popcoins"),
					Balance: uint64(1000000),
				}
				cciBuf, err := protobuf.Encode(&cci)
				if err != nil {
					return nil, nil, errors.New("couldn't encode CoinInstance: " + err.Error())
				}
				scs = append(scs, ol.NewStateChange(ol.Create, ol.NewInstanceID(iid.Sum(nil)),
					contracts.ContractCoinID, cciBuf, d.GetBaseID()))
			}
			return scs, coins, nil
		case "AddParty":
			return nil, nil, errors.New("not yet implemented")
		default:
			return nil, nil, errors.New("can only finalize Pop-party contract")
		}

	default:
		return nil, nil, errors.New("Only invoke and spawn are defined yet")
	}
}

func (s *Service) ContractPopCoinAccount(cdb ol.CollectionView, inst ol.Instruction, coins []ol.Coin) (sc []ol.StateChange, c []ol.Coin, err error) {
	return nil, coins, nil
}
