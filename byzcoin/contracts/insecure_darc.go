package contracts

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"golang.org/x/xerrors"
)

// ContractInsecureDarcID denotes a darc-contract
const ContractInsecureDarcID = "insecure_darc"

type contractInsecureDarc struct {
	byzcoin.BasicContract
	darc.Darc
	contracts byzcoin.ReadOnlyContractRegistry
}

var _ byzcoin.Contract = (*contractInsecureDarc)(nil)

func contractInsecureDarcFromBytes(in []byte) (byzcoin.Contract, error) {
	d, err := darc.NewFromProtobuf(in)
	if err != nil {
		return nil, err
	}
	c := &contractInsecureDarc{Darc: *d}
	return c, nil
}

// SetRegistry keeps the reference of the contract registry.
func (c *contractInsecureDarc) SetRegistry(r byzcoin.ReadOnlyContractRegistry) {
	c.contracts = r
}

func (c *contractInsecureDarc) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	if inst.Spawn.ContractID == ContractInsecureDarcID {
		darcBuf := inst.Spawn.Args.Search("darc")
		d, err := darc.NewFromProtobuf(darcBuf)
		if err != nil {
			return nil, nil, xerrors.Errorf("given darc could not be decoded: %v", err)
		}
		if d.Version != 0 {
			return nil, nil, xerrors.New("DARC version must start at 0")
		}
		id := d.GetBaseID()
		return []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Create, byzcoin.NewInstanceID(id), ContractInsecureDarcID, darcBuf, id),
		}, coins, nil
	}

	// If we got here this is a spawn:xxx in order to spawn
	// a new instance of contract xxx, so do that.

	if c.contracts == nil {
		return nil, nil, xerrors.New("contracts registry is missing due to bad initialization")
	}

	cfact, found := c.contracts.Search(inst.Spawn.ContractID)
	if !found {
		return nil, nil, xerrors.New("couldn't find this contract type: " + inst.Spawn.ContractID)
	}

	// Pass nil into the contract factory here because this instance does not exist yet.
	// So the factory will make a zero-value instance, and then calling Spawn on it
	// will give it a chance to encode it's zero state and emit one or more StateChanges to put itself
	// into the trie.
	c2, err := cfact(nil)
	if err != nil {
		return nil, nil, xerrors.Errorf("could not spawn new zero instance: %v", err)
	}
	return c2.Spawn(rst, inst, coins)
}

func (c *contractInsecureDarc) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	switch inst.Invoke.Command {
	case "evolve":
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
		oldD, err := rst.LoadDarcFromTrie(darcID)
		if err != nil {
			return nil, nil, err
		}
		if err := newD.SanityCheck(oldD); err != nil {
			return nil, nil, err
		}
		return []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID, ContractInsecureDarcID, darcBuf, darcID),
		}, coins, nil
	default:
		return nil, nil, xerrors.New("invalid command: " + inst.Invoke.Command)
	}
}
