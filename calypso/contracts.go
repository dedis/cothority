package calypso

import (
	"fmt"
	"strings"

	"go.dedis.ch/cothority/v4"
	"go.dedis.ch/cothority/v4/byzcoin"
	"go.dedis.ch/cothority/v4/darc"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// ContractWriteID references a write contract system-wide.
const ContractWriteID = "calypsoWrite"

// ContractWrite represents one calypso write instance.
type ContractWrite struct {
	byzcoin.BasicContract
	Write
}

// String returns a human readable string representation of the Write data
func (w Write) String() string {
	out := new(strings.Builder)
	out.WriteString("- Write:\n")
	fmt.Fprintf(out, "-- Data: %s\n", w.Data)
	fmt.Fprintf(out, "-- U: %s\n", w.U)
	fmt.Fprintf(out, "-- Ubar: %s\n", w.Ubar)
	fmt.Fprintf(out, "-- E: %s\n", w.E)
	fmt.Fprintf(out, "-- F: %s\n", w.F)
	fmt.Fprintf(out, "-- C: %s\n", w.C)
	fmt.Fprintf(out, "-- ExtraData: %s\n", w.ExtraData)
	fmt.Fprintf(out, "-- LTSID: %s\n", w.LTSID)
	fmt.Fprintf(out, "-- Cost: %x\n", w.Cost)

	return out.String()
}

func contractWriteFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &ContractWrite{}

	err := protobuf.DecodeWithConstructors(in, &c.Write, network.DefaultConstructors(cothority.Suite))
	return c, cothority.ErrorOrNil(err, "couldn't unmarshal write")
}

// Spawn is used to create a new write- or read-contract. The read-contract is
// created by the write-instance, because the creation of a new read-instance is
// protected by the write-contract's darc.
func (c ContractWrite) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		err = xerrors.Errorf("getting values: %v", err)
		return
	}

	switch inst.Spawn.ContractID {
	case ContractWriteID:
		w := inst.Spawn.Args.Search("write")
		if w == nil || len(w) == 0 {
			err = xerrors.New("need a write request in 'write' argument")
			return
		}
		err = protobuf.DecodeWithConstructors(w, &c.Write, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			err = xerrors.New("couldn't unmarshal write: " + err.Error())
			return
		}
		if d := inst.Spawn.Args.Search("darcID"); d != nil {
			darcID = d
		}
		if err = c.Write.CheckProof(cothority.Suite, darcID); err != nil {
			err = xerrors.Errorf("proof of write failed: %v", err)
			return
		}
		instID := inst.DeriveID("")
		log.Lvlf3("Successfully verified write request and will store in %x", instID)
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, instID, ContractWriteID, w, darcID))
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
		if c.Cost.Value > 0 {
			for i, coin := range cout {
				if coin.Name.Equal(c.Cost.Name) {
					err := coin.SafeSub(c.Cost.Value)
					if err != nil {
						return nil, nil, xerrors.Errorf("couldn't pay for read request: %v", err)
					}
					cout[i] = coin
					break
				}
			}
		}
		sc = byzcoin.StateChanges{byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""), ContractReadID, r, darcID)}
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

// ContractLongTermSecretID is the contract ID for updating the LTS roster.
var ContractLongTermSecretID = "longTermSecret"

type contractLTS struct {
	byzcoin.BasicContract
	LtsInstanceInfo LtsInstanceInfo
}

func contractLTSFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &contractLTS{}

	err := protobuf.DecodeWithConstructors(in, &c.LtsInstanceInfo, network.DefaultConstructors(cothority.Suite))
	return c, cothority.ErrorOrNil(err, "couldn't unmarshal LtsInfo")
}

func (c *contractLTS) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) ([]byzcoin.StateChange, []byzcoin.Coin, error) {
	var darcID darc.ID
	_, _, _, darcID, err := rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, xerrors.Errorf("getting values: %v", err)
	}

	if inst.Spawn.ContractID != ContractLongTermSecretID {
		return nil, nil, xerrors.New("can only spawn long-term-secret instances")
	}
	infoBuf := inst.Spawn.Args.Search("lts_instance_info")
	if infoBuf == nil || len(infoBuf) == 0 {
		return nil, nil, xerrors.New("need a lts_instance_info argument")
	}
	var info LtsInstanceInfo
	err = protobuf.DecodeWithConstructors(infoBuf, &info, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, nil, xerrors.Errorf("passed lts_instance_info argument is invalid: %v", err)
	}
	return byzcoin.StateChanges{byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""), ContractLongTermSecretID, infoBuf, darcID)}, coins, nil
}

func (c *contractLTS) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) ([]byzcoin.StateChange, []byzcoin.Coin, error) {
	var darcID darc.ID
	curBuf, _, _, darcID, err := rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, xerrors.Errorf("getting values: %v", err)
	}

	if inst.Invoke.Command != "reshare" {
		return nil, nil, xerrors.New("can only reshare long-term secrets")
	}
	infoBuf := inst.Invoke.Args.Search("lts_instance_info")
	if infoBuf == nil || len(infoBuf) == 0 {
		return nil, nil, xerrors.New("need a lts_instance_info argument")
	}

	var curInfo, newInfo LtsInstanceInfo
	err = protobuf.DecodeWithConstructors(infoBuf, &newInfo, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, nil, xerrors.Errorf("passed lts_instance_info argument is invalid: %v", err)
	}
	err = protobuf.DecodeWithConstructors(curBuf, &curInfo, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, nil, xerrors.Errorf("current info is invalid: %v", err)
	}

	// Verify the intersection between new roster and the old one. There must be
	// at least a threshold of nodes in the intersection.
	n := len(curInfo.Roster.List)
	overlap := intersectRosters(&curInfo.Roster, &newInfo.Roster)
	thr := n - (n-1)/3
	if overlap < thr {
		return nil, nil, xerrors.New("new roster does not overlap enough with current roster")
	}

	return byzcoin.StateChanges{byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID, ContractLongTermSecretID, infoBuf, darcID)}, coins, nil
}

func intersectRosters(r1, r2 *onet.Roster) int {
	res := 0
	for _, x := range r2.List {
		if i, _ := r1.Search(x.ID); i != -1 {
			res++
		}
	}
	return res
}
