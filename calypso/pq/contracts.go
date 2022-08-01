package pq

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// ContractPQWriteID references a system-wide contract for PQ-OTS.
const ContractPQWriteID = "calypsoPQWrite"

// ContractPQWrite represents one calypso pq-write instance.
type ContractPQWrite struct {
	byzcoin.BasicContract
	Write
}

func contractPQWriteFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &ContractPQWrite{}
	err := protobuf.DecodeWithConstructors(in, &c.Write, network.DefaultConstructors(cothority.Suite))
	return c, cothority.ErrorOrNil(err, "couldn't unmarshal write")
}
func (c ContractPQWrite) Spawn(rst byzcoin.ReadOnlyStateTrie,
	inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		err = xerrors.Errorf("getting values: %v", err)
		return
	}

	switch inst.Spawn.ContractID {
	case ContractPQWriteID:
		wrb := inst.Spawn.Args.Search("writereq")
		if wrb == nil || len(wrb) == 0 {
			err = xerrors.New("need a write req in 'writereq' argument")
			return
		}
		var req WriteTxn
		err = protobuf.DecodeWithConstructors(wrb, &req,
			network.DefaultConstructors(cothority.Suite))
		if err != nil {
			err = xerrors.New("couldn't unmarshal write req: " + err.Error())
			return
		}
		if d := inst.Spawn.Args.Search("darcID"); d != nil {
			darcID = d
		}
		if err = req.CheckSignatures(cothority.Suite); err != nil {
			err = xerrors.Errorf("Verifying write failed: %v", err)
			return
		}
		instID, err := inst.DeriveIDArg("", "preID")
		if err != nil {
			return nil, nil, xerrors.Errorf(
				"couldn't get ID for instance: %v", err)
		}
		log.Lvlf3("Successfully verified write request and will store in %x", instID)
		wb, err := protobuf.Encode(&req.Write)
		if err != nil {
			return nil, nil, xerrors.Errorf("couldn't encode write: %v", err)
		}
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, instID,
			ContractPQWriteID, wb, darcID))
	case ContractReadID:
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
			instID, ContractReadID, r, darcID)}
	default:
		err = xerrors.New("can only spawn writes and reads")
	}
	return
}

// ContractReadID references a read contract system-wide.
const ContractReadID = "calypsoRead"

// ContractRead represents one read contract.
type ContractRead struct {
	byzcoin.BasicContract
	Read
}

func contractReadFromBytes(in []byte) (byzcoin.Contract, error) {
	return nil, xerrors.New("calypso read instances are never instantiated")
}
