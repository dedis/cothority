package pqots

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// ContractPQOTSWriteID references a system-wide contract for PQ-OTS.
const ContractPQOTSWriteID = "calypsoPQOTSWrite"

// ContractPQOTSWrite represents one calypso pqots-write instance.
type ContractPQOTSWrite struct {
	byzcoin.BasicContract
	//Write
	WriteTxn
}

func contractPQOTSWriteFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &ContractPQOTSWrite{}
	//err := protobuf.DecodeWithConstructors(in, &c.Write, network.DefaultConstructors(cothority.Suite))
	err := protobuf.DecodeWithConstructors(in, &c.Write,
		network.DefaultConstructors(cothority.Suite))
	return c, cothority.ErrorOrNil(err, "couldn't unmarshal write")
}
func (c ContractPQOTSWrite) Spawn(rst byzcoin.ReadOnlyStateTrie,
	inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		err = xerrors.Errorf("getting values: %v", err)
		return
	}

	switch inst.Spawn.ContractID {
	case ContractPQOTSWriteID:
		buf := inst.Spawn.Args.Search("writetxn")
		if buf == nil || len(buf) == 0 {
			err = xerrors.New("need a write txn in 'writetxn' argument")
			return
		}
		//var wTxn WriteTxn
		//err = protobuf.DecodeWithConstructors(buf, &wTxn,
		//	network.DefaultConstructors(cothority.Suite))
		err = protobuf.DecodeWithConstructors(buf, &c.WriteTxn,
			network.DefaultConstructors(cothority.Suite))
		if err != nil {
			err = xerrors.New("couldn't unmarshal write txn: " + err.Error())
			return
		}
		if err = c.WriteTxn.CheckSignatures(cothority.Suite); err != nil {
			//if err = wTxn.CheckSignatures(cothority.Suite); err != nil {
			err = xerrors.Errorf("Verifying write failed: %v", err)
			return
		}
		if d := inst.Spawn.Args.Search("darcID"); d != nil {
			darcID = d
		}
		instID, err := inst.DeriveIDArg("", "preID")
		if err != nil {
			return nil, nil, xerrors.Errorf(
				"couldn't get ID for instance: %v", err)
		}
		log.Lvlf3("Successfully verified write request and will store in %x", instID)
		//wb, err := protobuf.Encode(&wTxn.Write)
		wb, err := protobuf.Encode(&c.Write)
		if err != nil {
			return nil, nil, xerrors.Errorf("couldn't encode write: %v", err)
		}
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, instID,
			ContractPQOTSWriteID, wb, darcID))
	case ContractPQOTSReadID:
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
			instID, ContractPQOTSReadID, r, darcID)}
	default:
		err = xerrors.New("can only spawn writes and reads")
	}
	return
}

// ContractPQOTSReadID references a read contract system-wide.
const ContractPQOTSReadID = "calypsoPQOTSRead"

// ContractPQOTSRead represents one read contract.
type ContractPQOTSRead struct {
	byzcoin.BasicContract
	Read
}

func contractPQOTSReadFromBytes(in []byte) (byzcoin.Contract, error) {
	return nil, xerrors.New("calypso read instances are never instantiated")
}
