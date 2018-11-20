package calypso

import (
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// ContractWriteID references a write contract system-wide.
const ContractWriteID = "calypsoWrite"

type contractWr struct {
	byzcoin.BasicContract
	Write
}

func contractWriteFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &contractWr{}

	err := protobuf.DecodeWithConstructors(in, &c.Write, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, errors.New("couldn't unmarshal write: " + err.Error())
	}
	return c, nil
}

func (c *contractWr) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.Spawn.ContractID {
	case ContractWriteID:
		w := inst.Spawn.Args.Search("write")
		if w == nil || len(w) == 0 {
			err = errors.New("need a write request in 'write' argument")
			return
		}
		err = protobuf.DecodeWithConstructors(w, &c.Write, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			err = errors.New("couldn't unmarshal write: " + err.Error())
			return
		}
		if err = c.Write.CheckProof(cothority.Suite, darcID); err != nil {
			err = errors.New("proof of write failed: " + err.Error())
			return
		}
		instID := inst.DeriveID("")
		log.Lvlf3("Successfully verified write request and will store in %x", instID)
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, instID, ContractWriteID, w, darcID))

	case ContractReadID:
		var rd Read
		r := inst.Spawn.Args.Search("read")
		if r == nil || len(r) == 0 {
			return nil, nil, errors.New("need a read argument")
		}
		err = protobuf.DecodeWithConstructors(r, &rd, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return nil, nil, errors.New("passed read argument is invalid: " + err.Error())
		}
		_, _, wc, _, err := rst.GetValues(rd.Write.Slice())
		if err != nil {
			return nil, nil, errors.New("referenced write-id is not correct: " + err.Error())
		}
		if wc != ContractWriteID {
			return nil, nil, errors.New("referenced write-id is not a write instance, got " + wc)
		}
		sc = byzcoin.StateChanges{byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""), ContractReadID, r, darcID)}
	default:
		err = errors.New("can only spawn writes and reads")
	}
	return
}

// ContractReadID references a read contract system-wide.
const ContractReadID = "calypsoRead"

type contractRe struct {
	byzcoin.BasicContract
	Read
}

func contractReadFromBytes(in []byte) (byzcoin.Contract, error) {
	return nil, errors.New("calypso read instances are never instantiated")
}
