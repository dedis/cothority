package contracts

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/xerrors"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/group/edwards25519"
	"go.dedis.ch/kyber/v3/sign/anon"
	"go.dedis.ch/kyber/v3/xof/blake2xs"
	"go.dedis.ch/onet/v3/log"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// ContractPopPartyID represents a pop-party that can be in one of three states:
//   1 - configuration
//   2 - scanning
//   3 - finalized
var ContractPopPartyID = "popParty"

// ContractPopParty embeds the BasicContract to be able to verify the calling darc is respected.
type ContractPopParty struct {
	byzcoin.BasicContract
	PopPartyStruct
}

const (
	// InitState is the initial state of a party.
	InitState = iota + 1
	// ScanningState is the second state when organizers are scanning
	// attendees' public keys.
	ScanningState
	// FinalizedState is the final state of a party.
	FinalizedState
)

// ContractPopPartyFromBytes returns the ContractPopPary structure given a slice of bytes.
func ContractPopPartyFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &ContractPopParty{}
	err := protobuf.DecodeWithConstructors(in, &c.PopPartyStruct, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, errors.New("couldn't unmarshal existing PopPartyStruct: " + err.Error())
	}
	return c, nil
}

// VerifyInstruction overrides the basic VerifyInstruction in case of a "mine" command, because this command
// is not protected by a darc, but by a linkable ring signature.
func (c ContractPopParty) VerifyInstruction(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, ctxHash []byte) error {
	if inst.GetType() == byzcoin.InvokeType && inst.Invoke.Command == "mine" {
		log.Lvl2("not verifying darc for mining")
		return nil
	}
	return c.BasicContract.VerifyInstruction(rst, inst, ctxHash)
}

// Spawn creates a new pop party contract. The following arguments are needed:
//  - description holds a protobuf encoded 'Description'
//  - darcID holds the id of the darc responsible for the pop party
//  - miningReward defines how much the 'mine' command will put into a coin-account
func (c ContractPopParty) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction,
	coins []byzcoin.Coin) (scs []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	descBuf := inst.Spawn.Args.Search("description")
	if descBuf == nil {
		return nil, nil, errors.New("need description argument")
	}
	darcID := inst.Spawn.Args.Search("darcID")
	if darcID == nil {
		return nil, nil, errors.New("no darcID argument")
	}
	c.State = InitState

	err = protobuf.DecodeWithConstructors(descBuf, &c.Description, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, nil, errors.New("couldn't unmarshal the description: " + err.Error())
	}

	value, _, _, _, err := rst.GetValues(darcID)
	if err != nil {
		return nil, nil, errors.New("couldn't get darc in charge: " + err.Error())
	}
	d, err := darc.NewFromProtobuf(value)
	if err != nil {
		return nil, nil, errors.New("couldn't get darc: " + err.Error())
	}
	var expr expression.Expr
	if rst.GetVersion() < 3 {
		expr = d.Rules.Get("invoke:finalize")
	} else {
		expr = d.Rules.Get("invoke:popParty.finalize")
	}
	c.Organizers = len(strings.Split(string(expr), "|"))

	miningRewardBuf := inst.Spawn.Args.Search("miningReward")
	if miningRewardBuf == nil {
		return nil, nil, errors.New("no miningReward argument")
	}
	c.MiningReward = binary.LittleEndian.Uint64(miningRewardBuf)

	ppiBuf, err := protobuf.Encode(&c.PopPartyStruct)
	if err != nil {
		return nil, nil, errors.New("couldn't marshal PopPartyStruct: " + err.Error())
	}

	scs = byzcoin.StateChanges{
		byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""), ContractPopPartyID, ppiBuf, darcID),
	}
	return
}

// SuiteBlake2s represents an ed25519 group,
// but with a blake2xs (instead of blake2xb) xof.
type SuiteBlake2s struct {
	edwards25519.SuiteEd25519
}

// XOF uses the blake2xs, as the blake2xb is not readily available in javascript.
func (sb SuiteBlake2s) XOF(key []byte) kyber.XOF {
	return blake2xs.New(key)
}

// Invoke uses the following commands:
//  - barrier to activate the pop-party
//  - finalize to store the attendees. If all organizers finalize using the same list of attendees,
//    the party is finalized
//  - addParty to add a new party to the list - not supported yet
//  - mine to collect the reward. 'lrs' must hold a correct, unique linkable ring signature. If
//    'coinIID' is set, this coin will be filled. Else 'newDarc' will be used to create a darc,
//    derive a coin, and fill this coin.
func (c *ContractPopParty) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction,
	coins []byzcoin.Coin) (scs []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, errors.New("couldn't get instance data: " + err.Error())
	}

	switch inst.Invoke.Command {
	case "barrier":
		if c.State != InitState {
			return nil, nil, fmt.Errorf("can only start barrier point when in configuration mode")
		}
		c.State = ScanningState

	case "finalize":
		if c.State != ScanningState {
			return nil, nil, fmt.Errorf("can only finalize when barrier point is passed")
		}

		attBuf := inst.Invoke.Args.Search("attendees")
		if attBuf == nil {
			return nil, nil, errors.New("missing argument: attendees")
		}
		var atts Attendees
		err = protobuf.DecodeWithConstructors(attBuf, &atts, network.DefaultConstructors(cothority.Suite))
		log.Lvl2("Adding attendees:", atts.Keys)

		alreadySigned := false
		orgSigner := inst.SignerIdentities[0].String()
		for _, f := range c.Finalizations {
			if f == orgSigner {
				alreadySigned = true
				log.Lvl2("this organizer already sent a finalization - resetting list of attendees")
				break
			}
		}

		if len(c.Finalizations) == 0 || alreadySigned {
			// Store first proposition of list of attendees or reset if the same
			// organizer submits again
			c.Attendees = atts
			c.Finalizations = []string{orgSigner}
			log.Lvl2("resetting list of attendees")
		} else {
			// Check if it is the same set of attendees or not
			same := len(c.Attendees.Keys) == len(atts.Keys)
			if same {
				for i, att := range c.Attendees.Keys {
					if !att.Equal(atts.Keys[i]) {
						same = false
					}
				}
			}
			if same {
				log.Lvl2("one more finalization")
				c.Finalizations = append(c.Finalizations, orgSigner)
			} else {
				log.Lvl2("not the same list of attendees - resetting")
				c.Attendees = atts
				c.Finalizations = []string{orgSigner}
			}
		}
		if len(c.Finalizations) == c.Organizers {
			log.Lvlf2("Successfully finalized party %s / %x", c.Description.Name, inst.InstanceID[:])
			c.State = FinalizedState
		}

	case "mine":
		if c.State != FinalizedState {
			return nil, nil, errors.New("cannot mine when party is not finalized")
		}
		lrs := inst.Invoke.Args.Search("lrs")
		if lrs == nil {
			return nil, nil, errors.New("need lrs argument")
		}
		tag, err := anon.Verify(&SuiteBlake2s{}, []byte("mine"), c.Attendees.Keys, inst.InstanceID[:], lrs)
		if err != nil {
			return nil, nil, errors.New("error while verifying signature: " + err.Error())
		}
		for _, t := range c.Miners {
			if bytes.Compare(t.Tag, tag) == 0 {
				return nil, nil, errors.New("this attendee already mined")
			}
		}
		c.Miners = append(c.Miners, LRSTag{Tag: tag})

		var coin byzcoin.Coin
		var coinDarc darc.ID
		coinAction := byzcoin.Update
		coinIID := inst.Invoke.Args.Search("coinIID")
		if coinIID == nil {
			newDarcBuf := inst.Invoke.Args.Search("newDarc")
			if newDarcBuf == nil {
				return nil, nil, errors.New("need either coinIID or newDarc argument")
			}
			newDarc, err := darc.NewFromProtobuf(newDarcBuf)
			if err != nil {
				return nil, nil, errors.New("couldn't unmarshal darc: " + err.Error())
			}
			// Creating new darc for new user
			log.Lvlf2("Creating new darc %x for user", newDarc.GetBaseID())
			scs = append(scs, byzcoin.NewStateChange(byzcoin.Create,
				byzcoin.NewInstanceID(newDarc.GetBaseID()), byzcoin.ContractDarcID,
				newDarcBuf, darcID))
			coinAction = byzcoin.Create
			h := sha256.New()
			h.Write([]byte("coin"))
			h.Write(newDarc.GetBaseID())
			coinIID = h.Sum(nil)
			coinDarc = newDarc.GetBaseID()
			log.Lvlf2("Creating new coin %x for user", coinIID)
			coin.Name = byzcoin.NewInstanceID([]byte("SpawnerCoin"))
		} else {
			var cid string
			var coinBuf []byte
			coinBuf, _, cid, coinDarc, err = rst.GetValues(coinIID)
			if cid != contracts.ContractCoinID {
				return nil, nil, errors.New("coinIID is not a coin contract")
			}
			err = protobuf.Decode(coinBuf, &coin)
			if err != nil {
				return nil, nil, errors.New("couldn't unmarshal coin: " + err.Error())
			}
		}
		err = coin.SafeAdd(c.MiningReward)
		if err != nil {
			return nil, nil, errors.New("couldn't add mining reward: " + err.Error())
		}
		coinBuf, err := protobuf.Encode(&coin)
		if err != nil {
			return nil, nil, errors.New("couldn't encode coin: " + err.Error())
		}
		scs = append(scs, byzcoin.NewStateChange(coinAction,
			byzcoin.NewInstanceID(coinIID),
			contracts.ContractCoinID, coinBuf, coinDarc))

	default:
		return nil, nil, errors.New("unknown command: " + inst.Invoke.Command)
	}

	// Storing new version of PopPartyStruct
	ppiBuf, err := protobuf.Encode(&c.PopPartyStruct)
	if err != nil {
		return nil, nil, errors.New("couldn't marshal PopPartyStruct: " + err.Error())
	}

	// Update existing party structure
	scs = append(scs, byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID, ContractPopPartyID, ppiBuf, darcID))

	return scs, coins, nil
}

// NewInstructionPoppartySpawn returns a new instruction that is ready to be
// sent to byzcoin to spawn a new pop-party instance.
func NewInstructionPoppartySpawn(dst byzcoin.InstanceID, did darc.ID,
	desc PopDesc, reward uint64) (
	inst byzcoin.Instruction, err error) {

	inst.InstanceID = dst
	rewardBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(rewardBuf, reward)
	descBuf, err := protobuf.Encode(&desc)
	if err != nil {
		err = xerrors.Errorf("couldn't encode description: %+v", err)
	}
	inst.Spawn = &byzcoin.Spawn{
		ContractID: ContractCredentialID,
		Args: byzcoin.Arguments{
			newArg("darcID", did),
			newArg("description", descBuf),
			newArg("miningReward", rewardBuf),
		},
	}
	return
}
