package byzcoin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// NamingInstanceID is the instance ID for the singleton naming contract.
var NamingInstanceID = InstanceID([32]byte{1})

// NamingConfig holds the latest pointer of a linked list of naming entries.
type NamingConfig struct {
	Latest InstanceID
}

// ContractNamingID is the ID of the naming contract.
const ContractNamingID = "naming"

type contractNaming struct {
	BasicContract
	NamingConfig
}

func contractNamingFromBytes(in []byte) (Contract, error) {
	c := &contractNaming{}
	err := protobuf.DecodeWithConstructors(in, &c.NamingConfig, network.DefaultConstructors(cothority.Suite))

	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *contractNaming) VerifyInstruction(rst ReadOnlyStateTrie, inst Instruction, msg []byte) error {
	pr, err := rst.GetProof(NamingInstanceID.Slice())
	if err != nil {
		return err
	}
	ok, err := pr.Exists(NamingInstanceID.Slice())
	if err != nil {
		return err
	}

	// The naming contract does not exist yet, so we need to create a singleton.
	if !ok {
		return nil
	}

	// If this is not a genesis transaction, the authorization of this
	// contract is not controlled by its own darc. Instead, it is
	// controlled by the darc that guards the instance ID in the invoke
	// argument.

	// check the number of signers match with the number of signatures
	if len(inst.SignerIdentities) != len(inst.Signatures) {
		return errors.New("lengh of identities does not match the length of signatures")
	}
	if len(inst.Signatures) == 0 {
		return errors.New("no signatures - nothing to verify")
	}

	// check the signature counters
	if err := verifySignerCounters(rst, inst.SignerCounter, inst.SignerIdentities); err != nil {
		return err
	}

	// get the darc, we have to do it differently than the normal
	// verification because the darc that we are interested in is the darc
	// that guards the instance ID in the instruction
	if inst.Invoke == nil {
		// TODO this needs to be changed when we add delete
		return errors.New("only invoke is supported")
	}
	value := inst.Invoke.Args.Search("instanceID")
	if value == nil {
		return errors.New("argument instanceID is missing")
	}
	_, _, cID, dID, err := rst.GetValues(value)
	if err != nil {
		return err
	}
	d, err := LoadDarcFromTrie(rst, dID)
	if err != nil {
		return err
	}

	// check the action, again we do this differently because we only care
	// about the spawn part of the given instance ID
	action := "spawn:" + cID
	ex := d.Rules.Get(darc.Action(action))
	if len(ex) == 0 {
		return fmt.Errorf("action '%v' does not exist", action)
	}

	// check the signature
	// Save the identities that provide good signatures
	goodIdentities := make([]string, 0)
	for i := range inst.Signatures {
		if err := inst.SignerIdentities[i].Verify(msg, inst.Signatures[i]); err == nil {
			goodIdentities = append(goodIdentities, inst.SignerIdentities[i].String())
		}
	}

	// check the expression
	getDarc := func(str string, latest bool) *darc.Darc {
		if len(str) < 5 || string(str[0:5]) != "darc:" {
			return nil
		}
		darcID, err := hex.DecodeString(str[5:])
		if err != nil {
			return nil
		}
		d, err := LoadDarcFromTrie(rst, darcID)
		if err != nil {
			return nil
		}
		return d
	}
	return darc.EvalExpr(ex, getDarc, goodIdentities...)
}

func (c *contractNaming) Spawn(rst ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cout []Coin, err error) {
	cout = coins
	var buf []byte
	// For the very first pointer, we use the default InstanceID value.
	buf, err = protobuf.Encode(&NamingConfig{Latest: InstanceID{}})
	if err != nil {
		return
	}
	sc = []StateChange{
		// We do not need a darc ID because the verification works
		// differently than normal contracts. See the verification
		// function for more details.
		NewStateChange(Create, NamingInstanceID, ContractNamingID, buf, nil),
	}
	return
}

type namingValue struct {
	IID  InstanceID
	Prev InstanceID
}

func (c *contractNaming) Invoke(rst ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cout []Coin, err error) {
	cout = coins

	switch inst.Invoke.Command {
	case "add":
		iID := inst.Invoke.Args.Search("instanceID")
		var dID darc.ID
		_, _, _, dID, err = rst.GetValues(iID)
		if err != nil {
			return
		}

		// Construct the key.
		name := inst.Invoke.Args.Search("name")
		if len(name) == 0 {
			err = errors.New("the name cannot be empty")
			return
		}
		key := sha256.Sum256(append(append(dID, '/'), name...))

		// Construct the value.
		valueStruct := namingValue{
			IID:  NewInstanceID(iID),
			Prev: c.Latest,
		}
		var valueBuf []byte
		valueBuf, err = protobuf.Encode(&valueStruct)
		if err != nil {
			return
		}

		// Create the new naming contract buffer where the pointer to
		// the latest value is updated.
		var contractBuf []byte
		contractBuf, err = protobuf.Encode(&NamingConfig{Latest: key})
		if err != nil {
			return
		}

		// Create the state change.
		sc = []StateChange{
			NewStateChange(Create, NewInstanceID(key[:]), "", valueBuf, nil),
			NewStateChange(Update, NamingInstanceID, ContractNamingID, contractBuf, nil),
		}
		return
	// TODO case "remove":
	default:
		err = errors.New("invalid invoke command: " + inst.Invoke.Command)
		return
	}
	return nil, nil, nil
}
