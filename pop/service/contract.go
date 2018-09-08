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
	"github.com/dedis/kyber"
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

// PoPCoinName is the identifier of the popcoins.
var PoPCoinName ol.InstanceID

func init() {
	h := sha256.New()
	h.Write([]byte("popcoin"))
	PoPCoinName = ol.NewInstanceID(h.Sum(nil))
}

// ContractPopParty represents a pop-party on OmniLedger. It has the following
// functionalities:
//   * Spawn - takes a "FinalStatement" argument with the binary representation
//     of the final statement to store.
//   * Invoke - hast the following Command
//     * "Finalize" - that stores a final statement and doesn't let it be
//       changed afterwards. It will also create a darc for every attendee
//       and bind that darc to a coin account, putting an initial 1.000.000
//       popCoins in it.
//        This command has the following arguments:
//       * "FinalStatement" - mandatory, to give the new final statement. It
//         needs to be correctly finalized by the pop-service.
//       * "Service" - when given, will create a darc and a coin-account for
//         the service to use.
func (s *Service) ContractPopParty(cdb ol.CollectionView, inst ol.Instruction, coins []ol.Coin) (scs []ol.StateChange, cOut []ol.Coin, err error) {
	cOut = coins

	err = inst.VerifyDarcSignature(cdb)
	if err != nil {
		return
	}

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

			for i, pub := range fs.Attendees {
				log.Lvlf3("Creating darc for attendee %d %s", i, pub)
				d, sc, err := createDarc(darcID, pub)
				if err != nil {
					return nil, nil, err
				}
				scs = append(scs, sc)

				sc, err = createCoin(inst, d, pub, 1000000)
				if err != nil {
					return nil, nil, err
				}
				scs = append(scs, sc)
			}

			// And add a service if the argument is given
			sBuf := inst.Invoke.Args.Search("Service")
			if sBuf != nil {
				ppi.Service = cothority.Suite.Point()
				err = ppi.Service.UnmarshalBinary(sBuf)
				if err != nil {
					return nil, nil, errors.New("couldn't unmarshal point: " + err.Error())
				}
				log.Lvlf3("Appending service-darc and account for %s", ppi.Service)
				d, sc, err := createDarc(darcID, ppi.Service)
				if err != nil {
					return nil, nil, err
				}
				scs = append(scs, sc)
				sc, err = createCoin(inst, d, ppi.Service, 0)
				if err != nil {
					return nil, nil, err
				}
				scs = append(scs, sc)
			}

			ppiBuf, err := protobuf.Encode(&ppi)
			if err != nil {
				return nil, nil, errors.New("couldn't marshal PopPartyInstance: " + err.Error())
			}

			// Update existing final statement
			scs = append(scs, ol.NewStateChange(ol.Update, inst.InstanceID, ContractPopParty, ppiBuf, darcID))

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

func createDarc(darcID darc.ID, pub kyber.Point) (d *darc.Darc, sc ol.StateChange, err error) {
	id := darc.NewIdentityEd25519(pub)
	rules := darc.InitRules([]darc.Identity{id}, []darc.Identity{id})
	rules.AddRule(darc.Action("invoke:transfer"), expression.Expr(id.String()))
	d = darc.NewDarc(rules, []byte("Attendee darc for pop-party"))
	darcBuf, err := d.ToProto()
	if err != nil {
		err = errors.New("couldn't marshal darc: " + err.Error())
		return
	}
	log.Lvlf3("Final %x/%x", d.GetBaseID(), sha256.Sum256(darcBuf))
	sc = ol.NewStateChange(ol.Create, ol.NewInstanceID(d.GetBaseID()),
		ol.ContractDarcID, darcBuf, darcID)
	return
}

func createCoin(inst ol.Instruction, d *darc.Darc, pub kyber.Point, balance uint64) (sc ol.StateChange, err error) {
	iid := sha256.New()
	iid.Write(inst.InstanceID.Slice())
	pubBuf, err := pub.MarshalBinary()
	if err != nil {
		err = errors.New("couldn't marshal public key: " + err.Error())
		return
	}
	iid.Write(pubBuf)
	cci := ol.Coin{
		Name:  PoPCoinName,
		Value: balance,
	}
	cciBuf, err := protobuf.Encode(&cci)
	if err != nil {
		err = errors.New("couldn't encode CoinInstance: " + err.Error())
		return
	}
	coinID := iid.Sum(nil)
	log.Lvlf3("Creating account %x", coinID)
	return ol.NewStateChange(ol.Create, ol.NewInstanceID(coinID),
		contracts.ContractCoinID, cciBuf, d.GetBaseID()), nil
}
