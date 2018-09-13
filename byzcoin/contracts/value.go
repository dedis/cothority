package contracts

import (
	"errors"

	"github.com/dedis/cothority/byzcoin/darc"
	ol "github.com/dedis/cothority/byzcoin"
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
func ContractValue(cdb ol.CollectionView, inst ol.Instruction, c []ol.Coin) (sc []ol.StateChange, cOut []ol.Coin, err error) {
	cOut = c

	err = inst.VerifyDarcSignature(cdb)
	if err != nil {
		return
	}

	var darcID darc.ID
	_, _, darcID, err = cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.GetType() {
	case ol.SpawnType:
		return []ol.StateChange{
			ol.NewStateChange(ol.Create, inst.DeriveID(""),
				ContractValueID, inst.Spawn.Args.Search("value"), darcID),
		}, c, nil
	case ol.InvokeType:
		if inst.Invoke.Command != "update" {
			return nil, nil, errors.New("Value contract can only update")
		}
		return []ol.StateChange{
			ol.NewStateChange(ol.Update, inst.InstanceID,
				ContractValueID, inst.Invoke.Args.Search("value"), darcID),
		}, c, nil
	case ol.DeleteType:
		return ol.StateChanges{
			ol.NewStateChange(ol.Remove, inst.InstanceID, ContractValueID, nil, darcID),
		}, c, nil
	}
	return nil, nil, errors.New("didn't find any instruction")
}
