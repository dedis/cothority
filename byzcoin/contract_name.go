package byzcoin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// ContractNamingID is the ID of the naming contract. This contract is a
// singleton contract that is always created in the genesis block. One can only
// invoke the naming contract to create a mapping from a darc ID and name tuple
// to another instance ID. This contract helps with useability so that the
// client does not need to store instance IDs as long as they are named.
//
// To add a mapping, create an Invoke instruction to the NamingInstanceID with
// the add command. You muse provide two arguments. The first is the instance
// ID that you wish to name which must exist. The second is the name that you
// want to use which is a string and must not be empty. The instruction must be
// signed by the signer(s) that has the "_name" permission to spawn the to-be-named
// instance ID.
//
// To get back a named instance ID, you should use the byzcoin API -
// ResolveInstanceID. You need to provide a darc ID and the name. The darc ID
// is the one that "guards" the the instance.
const ContractNamingID = "naming"

// ContractNamingBody holds a reference of the latest naming entries. These
// entries form a reversed linked list of. It is possible to traverse the
// reversed linked list to find all the naming entries.
type ContractNamingBody struct {
	Latest InstanceID
}

// NamingInstanceID is the instance ID for the singleton naming contract.
var NamingInstanceID = InstanceID([32]byte{1})

type contractNaming struct {
	BasicContract
	ContractNamingBody
}

// String returns a human readable string representation of ContractNamingBody
func (c ContractNamingBody) String() string {
	out := new(strings.Builder)
	out.WriteString("- ContractNamingBody:\n")
	fmt.Fprintf(out, "-- Latest: %s\n", c.Latest)

	return out.String()
}

func contractNamingFromBytes(in []byte) (Contract, error) {
	c := &contractNaming{}
	err := protobuf.DecodeWithConstructors(in, &c.ContractNamingBody, network.DefaultConstructors(cothority.Suite))

	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *contractNaming) VerifyInstruction(rst ReadOnlyStateTrie, inst Instruction, msg []byte) error {
	pr, err := rst.GetProof(NamingInstanceID.Slice())
	if err != nil {
		return errors.New("failed to get proof of NamingInstanceID: " + err.Error())
	}
	ok, err := pr.Exists(NamingInstanceID.Slice())
	if err != nil {
		return errors.New("failed to see if proof exists: " + err.Error())
	}

	// The naming contract does not exist yet, so we need to create a
	// singleton. Just like the config contract, we do not return an error
	// because there is no need/possibility to verify it.
	if !ok {
		return nil
	}

	// If this is not a genesis transaction, the authorization of this
	// contract is not controlled by its own darc. Instead, it is
	// controlled by the darc that guards the instance ID in the invoke
	// argument.

	// Check the number of signers match with the number of signatures.
	if len(inst.SignerIdentities) != len(inst.Signatures) {
		return errors.New("lengh of identities does not match the length of signatures")
	}
	if len(inst.Signatures) == 0 {
		return errors.New("no signatures - nothing to verify")
	}

	// Check the signature counters.
	if err := verifySignerCounters(rst, inst.SignerCounter, inst.SignerIdentities); err != nil {
		return errors.New("failed to verify the counters: " + err.Error())
	}

	// Get the darc, we have to do it differently than the normal
	// verification because the darc that we are interested in is the darc
	// that guards the instance ID in the instruction.
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
		return fmt.Errorf("failed to get the rst values of %s: %s", value, err.Error())
	}
	d, err := LoadDarcFromTrie(rst, dID)
	if err != nil {
		return errors.New("failed to load darc from tries: " + err.Error())
	}

	// Check that the darc has the right permission to allow naming.
	action := "_name:" + cID
	ex := d.Rules.Get(darc.Action(action))
	if len(ex) == 0 {
		return fmt.Errorf("action '%v' does not exist", action)
	}

	// Save the identities that provide good signatures.
	goodIdentities := make([]string, 0)
	for i := range inst.Signatures {
		if err := inst.SignerIdentities[i].Verify(msg, inst.Signatures[i]); err == nil {
			goodIdentities = append(goodIdentities, inst.SignerIdentities[i].String())
		}
	}
	if len(goodIdentities) == 0 {
		return errors.New("all signatures failed to verify")
	}

	// Evaluate the expression using the good signatures.
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
	buf, err = protobuf.Encode(&ContractNamingBody{Latest: InstanceID{}})
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

type contractNamingEntry struct {
	// IID is the instance ID that is named.
	IID InstanceID
	// Prev is a reference to the previous entry. It is used to form a
	// "reversed" linked list which enables us to track all the named
	// instances.
	Prev InstanceID
	// Removed marks whether the name has been removed. A removed name
	// cannot be used later.
	Removed bool
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
		h := sha256.New()
		h.Write(dID)
		h.Write([]byte{'/'})
		h.Write(name)
		key := NewInstanceID(h.Sum(nil))

		// Check that we are not overwriting.
		var oldEntryBuf []byte
		oldEntryBuf, _, _, _, err = rst.GetValues(key.Slice())
		if err != errKeyNotSet {
			oldEntry := contractNamingEntry{}
			err = protobuf.Decode(oldEntryBuf, &oldEntry)
			if err != nil {
				return
			}
			if oldEntry.Removed {
				err = errors.New("cannot create a name that existed before")
				return
			}
			err = errors.New("this name already exists")
			return
		}

		// Construct the value.
		entry := contractNamingEntry{
			IID:  NewInstanceID(iID),
			Prev: c.Latest,
		}
		var entryBuf []byte
		entryBuf, err = protobuf.Encode(&entry)
		if err != nil {
			return
		}

		// Create the new naming contract buffer where the pointer to
		// the latest value is updated.
		var contractBuf []byte
		contractBuf, err = protobuf.Encode(&ContractNamingBody{Latest: key})
		if err != nil {
			return
		}

		// Create the state change.
		sc = []StateChange{
			NewStateChange(Create, key, "", entryBuf, nil),
			NewStateChange(Update, NamingInstanceID, ContractNamingID, contractBuf, nil),
		}
		return
	case "remove":
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
		h := sha256.New()
		h.Write(dID)
		h.Write([]byte{'/'})
		h.Write(name)
		key := NewInstanceID(h.Sum(nil))

		// Check that the name that we want to delete exists and is alive.
		var oldEntryBuf []byte
		oldEntryBuf, _, _, _, err = rst.GetValues(key.Slice())
		if err != nil {
			return
		}
		oldEntry := contractNamingEntry{}
		err = protobuf.Decode(oldEntryBuf, &oldEntry)
		if err != nil {
			return
		}
		if oldEntry.Removed {
			err = errors.New("this entry is already removed")
			return
		}

		// Construct the value.
		oldEntry.Removed = true
		var entryBuf []byte
		entryBuf, err = protobuf.Encode(&oldEntry)
		if err != nil {
			return
		}

		sc = []StateChange{
			NewStateChange(Update, key, "", entryBuf, nil),
		}
		return
	default:
		err = errors.New("invalid invoke command: " + inst.Invoke.Command)
		return
	}
}
