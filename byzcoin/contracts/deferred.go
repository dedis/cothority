package contracts

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/protobuf"
)

// The deferred contract allows a group of signers to agree on and sign a
// proposed transaction, the "proposed transaction".

// ContractDeferredID denotes a contract that can aggregate signatures for a
// "proposed" instruction
var ContractDeferredID = "deferred"

// DeferredData contains the specific data of a deferred contract
type DeferredData struct {
	ProposedTransaction byzcoin.ClientTransaction
	Timestamp           uint64
	ExpireSec           uint64
	Hash                []byte
}

type contractDeferred struct {
	byzcoin.BasicContract
	DeferredData
	s *byzcoin.Service
}

func (s *Service) contractDeferredFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &contractDeferred{s: s.byzService()}

	err := protobuf.Decode(in, &c.DeferredData)
	if err != nil {
		return nil, errors.New("couldn't unmarshal instance data: " + err.Error())
	}
	return c, nil
}

func (c *contractDeferred) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	// This method should do the following:
	//   1. Parse the input buffer
	//   2. Compute and store the transaction hash
	//   3. Save the data
	//
	// Spawn should have those input arguments:
	//   - proposedTransaction ClientTransaction
	//   - expireSec uint64
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	// 1. Reads and parses the input
	proposedTransaction := byzcoin.ClientTransaction{}
	err = protobuf.Decode(inst.Spawn.Args.Search("proposedTransaction"), &proposedTransaction)
	timestamp := uint64(time.Now().Unix())
	expireSec, err := strconv.ParseUint(string(inst.Spawn.Args.Search("expireSec")), 10, 64)
	if err != nil {
		return nil, nil, errors.New("couldn't convert expireSec: " + err.Error())
	}

	// 2. Computes the hash
	hash := hashDeferred(proposedTransaction.Instructions[0], timestamp)

	// 3. Saves the data
	data := DeferredData{
		proposedTransaction,
		timestamp,
		expireSec,
		hash,
	}
	var dataBuf []byte
	dataBuf, err = protobuf.Encode(&data)
	if err != nil {
		return nil, nil, errors.New("couldn't encode DeferredData: " + err.Error())
	}

	sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""),
		ContractDeferredID, dataBuf, darcID))
	return
}

func (c *contractDeferred) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	// This method should do the following:
	//   - Handle the "addProof" invocation
	//   - Handle the "execProposedTx" invocation
	//
	// Invoke:addProof should have the following input argument:
	//   - identity darc.Identity
	//   - signature string
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID

	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.Invoke.Command {
	case "addProof":
		// This invocation appends the identity and the corresponding signature,
		// which is based on the stored hash of a client. Returns the contract's
		// data.

		// Get the given Identity
		identityBuf := inst.Invoke.Args.Search("identity")
		if identityBuf == nil {
			return nil, nil, errors.New("Identity args is nil")
		}
		identity := darc.Identity{}
		err = protobuf.Decode(identityBuf, &identity)
		if err != nil {
			return nil, nil, errors.New("Couldn't decode Identity")
		}
		// Get the given signature
		signature := inst.Invoke.Args.Search("signature")
		if signature == nil {
			return nil, nil, errors.New("Signature args is nil")
		}
		// Update the contract's data with the given signature and identity
		c.DeferredData.ProposedTransaction.Instructions[0].SignerIdentities = append(c.DeferredData.ProposedTransaction.Instructions[0].SignerIdentities, identity)
		c.DeferredData.ProposedTransaction.Instructions[0].Signatures = append(c.DeferredData.ProposedTransaction.Instructions[0].Signatures, signature)
		// Save and send the modifications
		cosiDataBuf, err2 := protobuf.Encode(&c.DeferredData)
		if err2 != nil {
			return nil, nil, errors.New("Couldn't encode DeferredData")
		}
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
			ContractDeferredID, cosiDataBuf, darcID))
		return
	case "execProposedTx":
		// This invocation tries to execute the transaction stored with the
		// "Spawn" invocation. If it is successful, this invocation returns
		// the InstanceID of the executed proposed transaction.

		instruction := c.DeferredData.ProposedTransaction.Instructions[0]

		// In case it goes well, we want to return the proposed Tx InstanceID
		rootInstructionID := instruction.DeriveID("").Slice()
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
			ContractDeferredID, rootInstructionID, darcID))

		instructionType := instruction.GetType()
		if instructionType == byzcoin.SpawnType {
			fn, exists := c.s.GetContractConstructor(instruction.Spawn.ContractID)
			if !exists {
				return nil, nil, errors.New("Couldn't get the root function")
			}
			rootInstructionBuff, err := protobuf.Encode(&c.DeferredData.ProposedTransaction.Instructions[0])
			if err != nil {
				return nil, nil, errors.New("Couldn't encode the root instruction buffer")
			}
			contract, err := fn(rootInstructionBuff)
			if err != nil {
				return nil, nil, errors.New("Couldn't get the root contract")
			}

			err = contract.VerifyDeferedInstruction(rst, instruction, c.DeferredData.Hash)
			if err != nil {
				return nil, nil, fmt.Errorf("Verifying the root instruction failed: %s", err)
			}

			rootSc, _, err := contract.Spawn(rst, c.DeferredData.ProposedTransaction.Instructions[0], coins)
			sc = append(sc, rootSc...)
		}
		return
	default:
		return nil, nil, errors.New("Cosi contract can only addProof and execRoot")
	}
}

func (c *contractDeferred) Delete(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	sc = byzcoin.StateChanges{
		byzcoin.NewStateChange(byzcoin.Remove, inst.InstanceID, ContractDeferredID, nil, darcID),
	}
	return
}

// This is a modified version of computing the hash of a transaction. In this
// version, we do not take into account the signers nor the signers counters. We
// also add to the hash a timestamp.
func hashDeferred(instr byzcoin.Instruction, timestamp uint64) []byte {
	h := sha256.New()
	h.Write(instr.InstanceID[:])
	var args []byzcoin.Argument
	switch instr.GetType() {
	case byzcoin.SpawnType:
		h.Write([]byte{0})
		h.Write([]byte(instr.Spawn.ContractID))
		args = instr.Spawn.Args
	case byzcoin.InvokeType:
		h.Write([]byte{1})
		h.Write([]byte(instr.Invoke.ContractID))
		args = instr.Invoke.Args
	case byzcoin.DeleteType:
		h.Write([]byte{2})
		h.Write([]byte(instr.Delete.ContractID))
	}
	for _, a := range args {
		nameBuf := []byte(a.Name)
		nameLenBuf := make([]byte, 8)
		binary.LittleEndian.PutUint64(nameLenBuf, uint64(len(nameBuf)))
		h.Write(nameLenBuf)
		h.Write(nameBuf)

		valueLenBuf := make([]byte, 8)
		binary.LittleEndian.PutUint64(valueLenBuf, uint64(len(a.Value)))
		h.Write(valueLenBuf)
		h.Write(a.Value)
	}
	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, timestamp)
	h.Write(timestampBuf)

	return h.Sum(nil)
}
