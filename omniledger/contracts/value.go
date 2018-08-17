package contracts

import (
	"errors"

	"github.com/dedis/cothority/omniledger/darc"
	omniledger "github.com/dedis/cothority/omniledger/service"
)

// The value contract can simply store a value in an instance and serves
// mainly as a template for other contracts. It helps show the possibilities
// of the contracts and how to use them at a very simple example.

// ContractValueID denotes a contract that can store and update
// key values.
var ContractValueID = "value"

// ContractValue is a simple key/value storage where you
// can put any data inside as wished.
// It can spawn new value instances and will store the "value" argument in these
// new instances.
// Existing value instances can be "update"d and deleted.
func ContractValue(cdb omniledger.CollectionView, inst omniledger.Instruction, c []omniledger.Coin) (sc []omniledger.StateChange, cOut []omniledger.Coin, err error) {
	cOut = c

	var darcID darc.ID
	_, _, darcID, err = cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch {
	case inst.Spawn != nil:
		return []omniledger.StateChange{
			omniledger.NewStateChange(omniledger.Create, omniledger.NewInstanceID(inst.Hash()),
				ContractValueID, inst.Spawn.Args.Search("value"), darcID),
		}, c, nil
	case inst.Invoke != nil:
		if inst.Invoke.Command != "update" {
			return nil, nil, errors.New("Value contract can only update")
		}
		return []omniledger.StateChange{
			omniledger.NewStateChange(omniledger.Update, inst.InstanceID,
				ContractValueID, inst.Invoke.Args.Search("value"), darcID),
		}, c, nil
	case inst.Delete != nil:
		return omniledger.StateChanges{
			omniledger.NewStateChange(omniledger.Remove, inst.InstanceID, ContractValueID, nil, darcID),
		}, c, nil
	}
	return nil, nil, errors.New("didn't find any instruction")
}
