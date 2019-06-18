package byzcoin

import (
	"crypto/sha256"
	"errors"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// NamingInstanceID is the instance ID for the singleton naming contract.
var NamingInstanceID = InstanceID([32]byte{1})

type NamingConfig struct {
	Dummy string
}

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

	// The authorization of this contract is not controlled by its own
	// darc. Instead, it is controlled by the darc that guards the instance
	// ID in the invoke argument. So we just check the counter and the
	// signature for now.

	// check the number of signers match with the number of signatures
	if len(inst.SignerIdentities) != len(inst.Signatures) {
		return errors.New("lengh of identities does not match the length of signatures")
	}

	// only one signer is supported
	if len(inst.SignerIdentities) != 1 {
		return errors.New("only one signer is supported")
	}

	// check the signature counters
	if err := verifySignerCounters(rst, inst.SignerCounter, inst.SignerIdentities); err != nil {
		return err
	}

	return inst.SignerIdentities[0].Verify(msg, inst.Signatures[0])
}

func (c *contractNaming) Spawn(rst ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cout []Coin, err error) {
	cout = coins
	var buf []byte
	buf, err = protobuf.Encode(&NamingConfig{"hello"})
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

func (c *contractNaming) Invoke(rst ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cout []Coin, err error) {
	cout = coins

	switch inst.Invoke.Command {
	case "add":
		// We only support the primary identity. The length is one
		// because we checked it in the verification function.
		if !inst.SignerIdentities[0].PrimaryIdentity() {
			err = errors.New("only the primary identity is supported")
			return
		}

		// Check that the value exists and the signer is authorised to
		// do the spawn.
		value := inst.Invoke.Args.Search("instanceID")
		{
			var cID string
			var dID darc.ID
			_, _, cID, dID, err = rst.GetValues(value)
			if err != nil {
				return
			}
			var d *darc.Darc
			d, err = LoadDarcFromTrie(rst, dID)
			if err != nil {
				return
			}
			action := "spawn:" + cID
			ex := d.Rules.Get(darc.Action(action))
			if len(ex) == 0 {
				err = errors.New("action: " + action + " does not exist")
				return
			}
			// getDarc always fails (returns nil) because we are
			// always evaluating the expression on a primary
			// identity.
			getDarc := func(s string, latest bool) *darc.Darc {
				return nil
			}
			err = darc.EvalExpr(ex, getDarc, inst.SignerIdentities[0].String())
			if err != nil {
				return
			}
		}

		// Construct the key.
		// TODO domain separation?
		name := inst.Invoke.Args.Search("name")
		if len(name) == 0 {
			err = errors.New("the name cannot be empty")
			return
		}
		namespace := inst.SignerIdentities[0].GetPublicBytes()
		// TODO add a delimiter like / between the namespace and the name
		key := sha256.Sum256(append(namespace, name...))

		// TODO check whether the key already exists? or leave it for
		// the contract executor to check

		// TODO put value in a struct and profobuf encode it
		sc = []StateChange{
			NewStateChange(Create, NewInstanceID(key[:]), "", value, nil),
		}
		return
	// case "remove":
	default:
		err = errors.New("invalid invoke command: " + inst.Invoke.Command)
		return
	}
	return nil, nil, nil
}
