package calypso

import (
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin/darc"
	ol "github.com/dedis/cothority/byzcoin/service"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// ContractWriteID references a write contract system-wide.
var ContractWriteID = "calypsoWrite"

// ContractWrite is used to store a secret in OmniLedger, so that an
// authorized reader can retrieve it by creating a Read-instance.
//
// Accepted Instructions:
//  - spawn:calypsoWrite creates a new write-request. TODO: verify the LTS exists
//  - spawn:calypsoRead creates a new read-request for this write-request.
func (s *Service) ContractWrite(cdb ol.CollectionView, inst ol.Instruction, c []ol.Coin) ([]ol.StateChange, []ol.Coin, error) {
	err := inst.VerifyDarcSignature(cdb)
	if err != nil {
		return nil, nil, err
	}

	var darcID darc.ID
	_, _, darcID, err = cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	switch inst.GetType() {
	case ol.SpawnType:
		var sc ol.StateChanges
		nc := c
		switch inst.Spawn.ContractID {
		case ContractWriteID:
			w := inst.Spawn.Args.Search("write")
			if w == nil || len(w) == 0 {
				return nil, nil, errors.New("need a write request in 'write' argument")
			}
			var wr Write
			err := protobuf.DecodeWithConstructors(w, &wr, network.DefaultConstructors(cothority.Suite))
			if err != nil {
				return nil, nil, errors.New("couldn't unmarshal write: " + err.Error())
			}
			if err = wr.CheckProof(cothority.Suite, darcID); err != nil {
				return nil, nil, errors.New("proof of write failed: " + err.Error())
			}
			instID := inst.DeriveID("")
			log.Lvlf3("Successfully verified write request and will store in %x", instID)
			sc = append(sc, ol.NewStateChange(ol.Create, instID, ContractWriteID, w, darcID))
		case ContractReadID:
			var scs ol.StateChanges
			var err error
			scs, nc, err = s.ContractRead(cdb, inst, c)
			if err != nil {
				return nil, nil, err
			}
			sc = append(sc, scs...)
		default:
			return nil, nil, errors.New("can only spawn writes and reads")
		}
		return sc, nc, nil
	default:
		return nil, nil, errors.New("asked for something we cannot do")
	}
}

// ContractReadID references a read contract system-wide.
var ContractReadID = "calypsoRead"

// ContractRead is used to create read instances that prove a reader has access
// to a given write instance. The following instructions are accepted:
//
//  - spawn:calypsoRead which does some health-checks to make sure that the read
//  request is valid.
//
// TODO: correctly handle multi signatures for read requests: to whom should the
// secret be re-encrypted to? Perhaps for multi signatures we only want to have
// ephemeral keys.
func (s *Service) ContractRead(cdb ol.CollectionView, inst ol.Instruction, c []ol.Coin) ([]ol.StateChange, []ol.Coin, error) {
	err := inst.VerifyDarcSignature(cdb)
	if err != nil {
		return nil, nil, err
	}

	var darcID darc.ID
	_, _, darcID, err = cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	switch inst.GetType() {
	case ol.SpawnType:
		if inst.Spawn.ContractID != ContractReadID {
			return nil, nil, errors.New("can only spawn read instances")
		}
		r := inst.Spawn.Args.Search("read")
		if r == nil || len(r) == 0 {
			return nil, nil, errors.New("need a read argument")
		}
		var re Read
		err := protobuf.DecodeWithConstructors(r, &re, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return nil, nil, errors.New("passed read argument is invalid: " + err.Error())
		}
		_, cid, _, err := cdb.GetValues(re.Write.Slice())
		if err != nil {
			return nil, nil, errors.New("referenced write-id is not correct: " + err.Error())
		}
		if cid != ContractWriteID {
			return nil, nil, errors.New("referenced write-id is not a write instance, got " + cid)
		}
		return ol.StateChanges{ol.NewStateChange(ol.Create, inst.DeriveID(""), ContractReadID, r, darcID)}, c, nil
	default:
		return nil, nil, errors.New("not a spawn instruction")
	}

}
