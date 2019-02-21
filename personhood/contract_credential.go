package personhood

import (
	"errors"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// ContractCredentialID denotes a contract that holds an identity with all its attributes.
var ContractCredentialID = "credential"

// ContractCredentialFromBytes returns a credential-contract given a slice of bytes, or an error if something
// went wrong.
func ContractCredentialFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &ContractCredential{}
	err := protobuf.Decode(in, &c.CredentialStruct)
	if err != nil {
		return nil, errors.New("couldn't unmarshal instance data: " + err.Error())
	}
	return c, nil
}

// ContractCredential structure embeds the BasicContract to verify the darc is correct.
type ContractCredential struct {
	byzcoin.BasicContract
	CredentialStruct
}

// Spawn creates a new credential contract and takes the following arguments:
//  - instID to set a static instanceID where to spawn the contract
//  - darcIDBuf to set which darc is responsible for the contract
//  - credential for the credential to be spawned.
func (c *ContractCredential) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	// Spawn creates a new credential as a separate instance.
	ca := inst.DeriveID("")
	if caBuf := inst.Spawn.Args.Search("instID"); caBuf != nil {
		ca = byzcoin.NewInstanceID(caBuf)
	}
	if darcIDBuf := inst.Spawn.Args.Search("darcIDBuf"); darcIDBuf != nil {
		darcID = darc.ID(darcIDBuf)
	}
	log.Lvlf3("Spawning Credential to %x", ca.Slice())
	var ciBuf []byte
	if ciBuf = inst.Spawn.Args.Search("credential"); ciBuf == nil {
		ciBuf, err = protobuf.Encode(&c.CredentialStruct)
		if err != nil {
			return nil, nil, errors.New("couldn't encode CredentialInstance: " + err.Error())
		}
	}
	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, ca, ContractCredentialID, ciBuf, darcID),
	}
	return
}

// Invoke has the following command:
//  - update to change the credential
func (c *ContractCredential) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.Invoke.Command {
	case "update":
		// update overwrites the credential information
		credBuf := inst.Invoke.Args.Search("credential")
		err = protobuf.Decode(credBuf, &c.CredentialStruct)
		if err != nil {
			return nil, nil, errors.New("got wrong credential data: " + err.Error())
		}

		sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
			ContractCredentialID, credBuf, darcID))
	default:
		err = errors.New("credential contract can only 'update'")
		return
	}
	return
}

// Delete removes a credential instance.
func (c *ContractCredential) Delete(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	sc = byzcoin.StateChanges{
		byzcoin.NewStateChange(byzcoin.Remove, inst.InstanceID, ContractCredentialID, nil, darcID),
	}
	return
}
