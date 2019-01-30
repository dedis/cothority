package contracts

import (
	"bytes"
	"errors"
	"fmt"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
)

// ContractSecureDarcID denotes a secure version of the DARC contract. We
// provide two forms of security. The first is "restricted evolution", where
// the evolve command only allows changes to existing rules, it is not allowed
// to add new rules. There exists an additional command "evolve_unrestricted"
// that allows authorised users to change the rules arbitrarily. Our second
// form of security is "controlled spawn", where the rules of the secure darcs
// spawned using this contract are subject to some restrictions, e.g., the new
// rules must not contain spawn:darc (the restricted DARC contract). This
// contract serves as an example. In practice, the contract developer will
// write his/her own contract for the intended application.
var ContractSecureDarcID = "secure_darc"

type contractSecureDarc struct {
	byzcoin.BasicContract
	darc.Darc
	s *byzcoin.Service
}

var _ byzcoin.Contract = (*contractSecureDarc)(nil)

const cmdDarcEvolveUnrestriction = "evolve_unrestricted"
const cmdDarcEvolve = "evolve"

func (s *Service) contractSecureDarcFromBytes(in []byte) (byzcoin.Contract, error) {
	d, err := darc.NewFromProtobuf(in)
	if err != nil {
		return nil, err
	}
	c := &contractSecureDarc{s: s.byzService(), Darc: *d}
	return c, nil
}

func (c *contractSecureDarc) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	if inst.Spawn.ContractID == byzcoin.ContractDarcID {
		return nil, nil, errors.New("this contract is not allowed to spawn a DARC instance")
	}

	if inst.Spawn.ContractID == ContractSecureDarcID {
		darcBuf := inst.Spawn.Args.Search("darc")
		d, err := darc.NewFromProtobuf(darcBuf)
		if err != nil {
			return nil, nil, errors.New("given DARC could not be decoded: " + err.Error())
		}
		id := d.GetBaseID()

		// Here is a hard-coded constraint for spawning DARCs. If the
		// constraint needs to be dynamic, then it is recommended to
		// create a new contract that contains mappings of roles ->
		// identities, and roles -> whitelist of rules. Then modify
		// this contract to check the whitelist.
		if d.Rules.Contains("spawn:darc") {
			return nil, nil, errors.New("a secure DARC is not allowed to spawn a regular DARC")
		}

		return []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Create, byzcoin.NewInstanceID(id), ContractSecureDarcID, darcBuf, id),
		}, coins, nil
	}

	// If we got here this is a spawn:xxx in order to spawn
	// a new instance of contract xxx, so do that.

	cfact, found := c.s.GetContractConstructor(inst.Spawn.ContractID)
	if !found {
		return nil, nil, errors.New("couldn't find this contract type: " + inst.Spawn.ContractID)
	}

	// Pass nil into the contract factory here because this instance does not exist yet.
	// So the factory will make a zero-value instance, and then calling Spawn on it
	// will give it a chance to encode it's zero state and emit one or more StateChanges to put itself
	// into the trie.
	c2, err := cfact(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("coult not spawn new zero instance: %v", err)
	}
	return c2.Spawn(rst, inst, coins)
}

func (c *contractSecureDarc) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	switch inst.Invoke.Command {
	case cmdDarcEvolve:
		var darcID darc.ID
		_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return
		}

		darcBuf := inst.Invoke.Args.Search("darc")
		newD, err := darc.NewFromProtobuf(darcBuf)
		if err != nil {
			return nil, nil, err
		}
		oldD, err := byzcoin.LoadDarcFromTrie(rst, darcID)
		if err != nil {
			return nil, nil, err
		}
		// do not allow modification of evolve_unrestricted
		if isChangingEvolveUnrestricted(oldD, newD) {
			return nil, nil, errors.New("the evolve command is not allowed to change the the evolve_unrestricted rule")
		}
		if err := newD.SanityCheck(oldD); err != nil {
			return nil, nil, err
		}
		// use the subset rule if it's not a genesis Darc
		_, _, _, genesisDarcID, err := byzcoin.GetValueContract(rst, byzcoin.NewInstanceID(nil).Slice())
		if err != nil {
			return nil, nil, err
		}
		if !genesisDarcID.Equal(oldD.GetBaseID()) {
			if !newD.Rules.IsSubset(oldD.Rules) {
				return nil, nil, errors.New("rules in the new version must be a subset of the previous version")
			}
		}
		return []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID, ContractSecureDarcID, darcBuf, darcID),
		}, coins, nil
	case cmdDarcEvolveUnrestriction:
		var darcID darc.ID
		_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return
		}

		darcBuf := inst.Invoke.Args.Search("darc")
		newD, err := darc.NewFromProtobuf(darcBuf)
		if err != nil {
			return nil, nil, err
		}
		oldD, err := byzcoin.LoadDarcFromTrie(rst, darcID)
		if err != nil {
			return nil, nil, err
		}
		if err := newD.SanityCheck(oldD); err != nil {
			return nil, nil, err
		}
		return []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID, ContractSecureDarcID, darcBuf, darcID),
		}, coins, nil
	default:
		return nil, nil, errors.New("invalid command: " + inst.Invoke.Command)
	}
}

func isChangingEvolveUnrestricted(oldD *darc.Darc, newD *darc.Darc) bool {
	oldExpr := oldD.Rules.Get(darc.Action(cmdDarcEvolveUnrestriction))
	newExpr := newD.Rules.Get(darc.Action(cmdDarcEvolveUnrestriction))
	if len(oldExpr) == 0 && len(newExpr) == 0 {
		return false
	}
	if bytes.Equal(oldExpr, newExpr) {
		return false
	}
	return true
}
