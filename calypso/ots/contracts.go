package ots

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// ContractOTSWriteID references a system-wide contract for OTS.
const ContractOTSWriteID = "calypsoOTSWrite"

// ContractOTSWrite represents one calypso ots-write instance.
type ContractOTSWrite struct {
	byzcoin.BasicContract
	Write
}

func contractOTSWriteFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &ContractOTSWrite{}
	err := protobuf.DecodeWithConstructors(in, &c.Write, network.DefaultConstructors(cothority.Suite))
	return c, cothority.ErrorOrNil(err, "couldn't unmarshal write")
}
func (c ContractOTSWrite) Spawn(rst byzcoin.ReadOnlyStateTrie,
	inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		err = xerrors.Errorf("getting values: %v", err)
		return
	}

	switch inst.Spawn.ContractID {
	case ContractOTSWriteID:
		wb := inst.Spawn.Args.Search("write")
		if wb == nil || len(wb) == 0 {
			err = xerrors.New("need a write req in 'write' argument")
			return
		}
		//var wr Write
		err = protobuf.DecodeWithConstructors(wb, &c.Write,
			network.DefaultConstructors(cothority.Suite))
		if err != nil {
			err = xerrors.New("couldn't unmarshal write: " + err.Error())
			return
		}
		if d := inst.Spawn.Args.Search("darcID"); d != nil {
			darcID = d
		}
		if !darcID.Equal(c.Write.PolicyID) {
			return nil, nil, xerrors.New("darcIDs do not match")
		}
		err := c.Write.VerifyWrite(cothority.Suite, darcID)
		if err != nil {
			return nil, nil, err
		}
		instID, err := inst.DeriveIDArg("", "preID")
		if err != nil {
			return nil, nil, xerrors.Errorf(
				"couldn't get ID for instance: %v", err)
		}
		log.Lvlf3("Successfully verified write request and will store in %x", instID)
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, instID,
			ContractOTSWriteID, wb, darcID))
	case ContractOTSReadID:
		var rd Read
		r := inst.Spawn.Args.Search("read")
		if r == nil || len(r) == 0 {
			return nil, nil, xerrors.New("need a read argument")
		}
		err = protobuf.DecodeWithConstructors(r, &rd, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return nil, nil, xerrors.Errorf("passed read argument is invalid: %v", err)
		}
		if !rd.Write.Equal(inst.InstanceID) {
			return nil, nil, xerrors.New("the read request doesn't reference this write-instance")
		}
		instID, err := inst.DeriveIDArg("", "preID")
		if err != nil {
			return nil, nil, xerrors.Errorf(
				"couldn't get ID for instance: %v", err)
		}
		sc = byzcoin.StateChanges{byzcoin.NewStateChange(byzcoin.Create,
			instID, ContractOTSReadID, r, darcID)}
	default:
		err = xerrors.New("can only spawn writes and reads")
	}
	return
}

// ContractOTSReadID references a read contract system-wide.
const ContractOTSReadID = "calypsoOTSRead"

// ContractOTSRead represents one read contract.
type ContractOTSRead struct {
	byzcoin.BasicContract
	Read
}

func contractOTSReadFromBytes(in []byte) (byzcoin.Contract, error) {
	return nil, xerrors.New("calypso read instances are never instantiated")
}
