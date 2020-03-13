package did

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/mr-tron/base58"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// ContractSovrinDIDID references the Sovrin DID contract
const ContractSovrinDIDID = "sovrinDID"

// ContractDID represents the DID contract.
type ContractDID struct {
	byzcoin.BasicContract
	Sovrin *Sovrin
	// Other DID methods may be placed here later
}

func init() {
	err := byzcoin.RegisterGlobalContract(ContractSovrinDIDID, contractSovrinDIDFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
}

func contractSovrinDIDFromBytes(data []byte) (byzcoin.Contract, error) {
	c := ContractDID{Sovrin: &Sovrin{}}
	if data == nil {
		return &c, nil
	}

	err := protobuf.DecodeWithConstructors(data, c.Sovrin, network.DefaultConstructors(cothority.Suite))
	return &c, cothority.ErrorOrNil(err, "error unmarshalling SovrinDID contract")
}

// Spawn is used to create a new SovrinDID contract.
func (c *ContractDID) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		err = xerrors.Errorf("getting values: %v", err)
		return
	}

	switch inst.Spawn.ContractID {
	case ContractSovrinDIDID:
		sovrin := inst.Spawn.Args.Search("sovrin")
		if sovrin == nil || len(sovrin) == 0 {
			err = xerrors.New("need information about sovrin to spawn the contract")
			return
		}
		c.Sovrin = &Sovrin{}
		err = protobuf.DecodeWithConstructors(sovrin, c.Sovrin, network.DefaultConstructors(cothority.Suite))
		instID := inst.DeriveID("")
		log.Lvlf3("Spawning %s contract with instance id: %s", ContractSovrinDIDID, instID)
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, instID, ContractSovrinDIDID, sovrin, darcID))
	default:
		err = xerrors.New("cannot spawn the given contract")
	}
	return
}

func (c *ContractDID) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) ([]byzcoin.StateChange, []byzcoin.Coin, error) {
	if inst.Invoke.Command != "set" {
		return nil, nil, xerrors.New("only the `set` command may be invoked")
	}

	if c.Sovrin != nil {
		return c.Sovrin.Invoke(rst, inst, coins)
	} else {
		return nil, nil, xerrors.Errorf("error invoking: invalid contract type")
	}
}

func (s *Sovrin) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) ([]byzcoin.StateChange, []byzcoin.Coin, error) {
	_, _, _, darcID, err := rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, xerrors.Errorf("error retrieving DARC: %v", err)
	}

	sovrinDIDPropsBuf := inst.Invoke.Args.Search("sovrinDIDProps")
	if sovrinDIDPropsBuf == nil || len(sovrinDIDPropsBuf) == 0 {
		return nil, nil, xerrors.New("did is a required argument")
	}
	var sovrinDIDProps SovrinDIDProps
	err = protobuf.Decode(sovrinDIDPropsBuf, &sovrinDIDProps)
	if err != nil {
		return nil, nil, xerrors.Errorf("error decoding arguments: %s", err)
	}

	if sovrinDIDProps.DID != sovrinDIDProps.Transaction.Dest {
		return nil, nil, xerrors.New("input did and the one referred in the transaction don't match")
	}

	var data getNymData
	json.Unmarshal([]byte(sovrinDIDProps.Transaction.Data), &data)
	if data.Verkey == "" {
		return nil, nil, xerrors.New("error parsing verkey")
	}

	didBuf, err := base58.Decode(sovrinDIDProps.DID)
	if err != nil {
		return nil, nil, xerrors.Errorf("error base58 decoding did: %s", err)
	}
	if len(didBuf) != 16 {
		return nil, nil, xerrors.New("expected a 16 byte DID")
	}

	// Sovrin DIDs are always fixed lengths and are therefore not vulnerable
	// to length extension attacks
	h := sha256.New()
	h.Write([]byte("did:sov:"))
	h.Write(didBuf)
	key := byzcoin.NewInstanceID(h.Sum(nil))

	pub := cothority.Suite.Point()
	var val []byte
	if data.Verkey[0] == '~' {
		verkeyBuf, err := base58.Decode(data.Verkey[1:])
		if err != nil {
			return nil, nil, xerrors.Errorf("error base58 decoding did: %s", err)
		}
		val = append(didBuf, verkeyBuf...)
	} else {
		val, err = base58.Decode(data.Verkey)
		if err != nil {
			return nil, nil, xerrors.Errorf("error base58 decoding verkey: %s", err)
		}
	}

	err = pub.UnmarshalBinary(val)
	if err != nil {
		return nil, nil, xerrors.Errorf("error unmarshalling verkey: %s", err)
	}

	didDoc := &darc.DIDDoc{
		ID: sovrinDIDProps.DID,
		PublicKeys: []darc.PublicKey{
			darc.PublicKey{
				ID:         fmt.Sprintf("did:sov:%s#keys-1", sovrinDIDProps.DID),
				Type:       "Ed25519VerificationKey2018",
				Controller: sovrinDIDProps.DID,
				Value:      val,
			},
		},
	}
	didDocBuf, err := protobuf.Encode(didDoc)
	if err != nil {
		return nil, nil, xerrors.Errorf("error encoding DID Doc: %v", err)
	}

	stateAction := byzcoin.Update
	_, _, _, _, err = rst.GetValues(key[:])

	// Key is not set
	if err != nil {
		stateAction = byzcoin.Create
	}

	log.Lvlf3("Setting key %x for did %s", key, sovrinDIDProps.DID)
	return []byzcoin.StateChange{byzcoin.NewStateChange(stateAction, key, ContractSovrinDIDID, didDocBuf, darcID)}, coins, nil
}
