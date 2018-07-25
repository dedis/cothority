package contracts

import (
	"errors"

	"github.com/dedis/cothority/omniledger/service"
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
func ContractValue(cdb service.CollectionView, inst service.Instruction, c []service.Coin) ([]service.StateChange, []service.Coin, error) {
	switch {
	case inst.Spawn != nil:
		return []service.StateChange{
			service.NewStateChange(service.Create, inst.DeriveID(ContractValueID),
				ContractValueID, inst.Spawn.Args.Search("value")),
		}, c, nil
	case inst.Invoke != nil:
		if inst.Invoke.Command != "update" {
			return nil, nil, errors.New("Value contract can only update")
		}
		return []service.StateChange{
			service.NewStateChange(service.Update, inst.InstanceID,
				ContractValueID, inst.Invoke.Args.Search("value")),
		}, c, nil
	case inst.Delete != nil:
		return service.StateChanges{
			service.NewStateChange(service.Remove, inst.InstanceID, ContractValueID, nil),
		}, c, nil
	}
	return nil, nil, errors.New("didn't find any instruction")
}
