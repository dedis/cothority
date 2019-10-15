package personhood

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"

	"go.dedis.ch/cothority/v4"
	"go.dedis.ch/cothority/v4/darc/expression"
	"go.dedis.ch/kyber/v4/sign/schnorr"

	"go.dedis.ch/cothority/v4/byzcoin"
	"go.dedis.ch/cothority/v4/darc"
	"go.dedis.ch/onet/v4/log"
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

// VerifyInstruction allows for an unsigned "recover" command that will be verified later.
func (c ContractCredential) VerifyInstruction(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, ctxHash []byte) error {
	// Because doing a threshold-definition using AND and OR in a darc can get very complex, this contract
	// does its own threshold verification and thus cannot rely on the darc to verify that the "recover" command
	// is valid.
	if inst.Invoke != nil && inst.Invoke.Command == "recover" {
		return nil
	}
	return c.BasicContract.VerifyInstruction(rst, inst, ctxHash)
}

// Spawn creates a new credential contract and takes the following arguments:
//  - darcIDBuf to set which darc is responsible for the contract
//  - credential for the credential to be spawned.
func (c *ContractCredential) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	// Warning: if you change this to darcID, then the current DEDIS-Byzcoin will fail, because it has
	// transactions with 'darcID' arguments in it.
	if darcIDBuf := inst.Spawn.Args.Search("darcIDBuf"); darcIDBuf != nil {
		darcID = darc.ID(darcIDBuf)
	} else {
		var cid string
		_, _, cid, darcID, err = rst.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return
		}
		if cid != byzcoin.ContractDarcID {
			return nil, nil, errors.New("give darcID if not spawned from a darc")
		}
	}

	// Spawn creates a new credential as a separate instance.
	ca := inst.DeriveID("")
	if credID := inst.Spawn.Args.Search("credentialID"); credID != nil {
		h := sha256.New()
		h.Write([]byte(ContractCredentialID))
		h.Write(credID)
		ca = byzcoin.NewInstanceID(h.Sum(nil))
	}
	log.Lvlf3("Spawning Credential to %x", ca.Slice())

	var ciBuf []byte
	if ciBuf = inst.Spawn.Args.Search("credential"); ciBuf == nil {
		ciBuf, err = protobuf.Encode(&c.CredentialStruct)
		if err != nil {
			return nil, nil, errors.New("couldn't encode CredentialInstance: " + err.Error())
		}
	} else {
		err = protobuf.Decode(ciBuf, &c.CredentialStruct)
		if err != nil {
			return nil, nil, errors.New("got wrong credential data: " + err.Error())
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

	case "recover":
		// "recover" checks if enough signatures are present to change the 'evolve' and 'sign' rule
		// of the darc attached to the credential.
		pointLength := cothority.Suite.PointLen()
		sigLength := cothority.Suite.ScalarLen() + cothority.Suite.PointLen()
		recoverLength := pointLength + sigLength
		iidLength := len(byzcoin.InstanceID{})

		recBuf := inst.Invoke.Args.Search("signatures")
		if len(recBuf) == 0 || len(recBuf)%recoverLength != 0 {
			return nil, nil, errors.New("wrong signatures argument")
		}
		pubBuf := inst.Invoke.Args.Search("public")
		if len(pubBuf) != pointLength {
			return nil, nil, errors.New("wrong 'public' argument")
		}
		public := cothority.Suite.Point()
		err = public.UnmarshalBinary(pubBuf)
		if err != nil {
			return
		}
		var d *darc.Darc
		d, err = getDarc(rst, darcID)
		if err != nil {
			return
		}
		var trusteesDarc []*darc.Darc
		var threshold uint32
		for _, cred := range c.Credentials {
			if cred.Name == "recover" {
				for _, att := range cred.Attributes {
					switch att.Name {
					case "threshold":
						threshold = binary.LittleEndian.Uint32(att.Value)
					case "trustees":
						for t := 0; t < len(att.Value); t += iidLength {
							trusteeDarc, err := getDarcFromCredIID(rst, att.Value[t:t+iidLength])
							if err != nil {
								return nil, nil, err
							}
							trusteesDarc = append(trusteesDarc, trusteeDarc)
						}
					default:
						return nil, nil, errors.New("unknown recover attribute: " + att.Name)
					}
				}
				break
			}
		}
		if threshold == 0 || len(trusteesDarc) == 0 {
			return nil, nil, errors.New("no threshold or no trustee found")
		}
		var valid uint32
		msg := append(inst.InstanceID.Slice(), pubBuf...)
		darcVersion := make([]byte, 8)
		binary.LittleEndian.PutUint64(darcVersion, d.Version)
		msg = append(msg, darcVersion...)
		for signer := 0; signer < len(recBuf); signer += sigLength {
			pubBuf := recBuf[signer : signer+pointLength]
			sig := recBuf[signer+pointLength : signer+sigLength]
			pub := cothority.Suite.Point()
			err = pub.UnmarshalBinary(pubBuf)
			if err != nil {
				return nil, nil, err
			}
			pubStr := darc.NewIdentityEd25519(pub).String()
			if err = schnorr.Verify(cothority.Suite, pub, msg, sig); err == nil {
				for _, trusteeDarc := range trusteesDarc {
					if err := checkDarcRule(rst, trusteeDarc, pubStr); err == nil {
						valid++
						break
					}
				}
			} else {
				log.Warn("Got invalid signature in recovery for public key", pubStr)
			}
		}
		if valid < threshold {
			return nil, nil, errors.New("didn't reach threshold for recovery")
		}
		publicStr := darc.NewIdentityEd25519(public).String()
		newDarc := d.Copy()
		err = newDarc.Rules.UpdateRule("invoke:evolve", expression.InitAndExpr(publicStr))
		if err != nil {
			return
		}
		err = newDarc.Rules.UpdateSign(expression.InitAndExpr(publicStr))
		if err != nil {
			return
		}
		err = newDarc.EvolveFrom(d)
		if err != nil {
			return
		}
		var newDarcBuf []byte
		newDarcBuf, err = newDarc.ToProto()
		if err != nil {
			return
		}
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, byzcoin.NewInstanceID(newDarc.GetBaseID()),
			byzcoin.ContractDarcID, newDarcBuf, newDarc.GetBaseID()))
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

func getDarc(rst byzcoin.ReadOnlyStateTrie, darcID darc.ID) (*darc.Darc, error) {
	darcBuf, _, cid, _, err := rst.GetValues(darcID)
	if err != nil {
		return nil, err
	}
	if cid != byzcoin.ContractDarcID {
		return nil, errors.New("this is not a darc-id")
	}
	return darc.NewFromProtobuf(darcBuf)
}

func getDarcFromCredIID(rst byzcoin.ReadOnlyStateTrie, credIID []byte) (*darc.Darc, error) {
	_, _, cid, darcID, err := rst.GetValues(credIID)
	if err != nil {
		return nil, err
	}
	if cid != ContractCredentialID {
		return nil, errors.New("not a credential instance")
	}
	return getDarc(rst, darcID)
}

func checkDarcRule(rst byzcoin.ReadOnlyStateTrie, d *darc.Darc, id string) error {
	getDarc := func(str string, latest bool) *darc.Darc {
		if strings.HasPrefix(str, "darc:") {
			return nil
		}
		darcID, err := hex.DecodeString(str[5:])
		if err != nil {
			return nil
		}
		d, err := byzcoin.LoadDarcFromTrie(rst, darcID)
		if err != nil {
			return nil
		}
		return d
	}
	return darc.EvalExpr(d.Rules.Get(darc.Action("_sign")), getDarc, id)
}
