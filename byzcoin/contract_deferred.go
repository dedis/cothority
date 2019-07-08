package byzcoin

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/protobuf"
)

// The deferred contract allows a group of signers to agree on and sign a
// proposed transaction, the "proposed transaction".

// ContractDeferredID denotes a contract that can aggregate signatures for a
// "proposed" transaction
var ContractDeferredID = "deferred"

// We originally implemented the feature where a client could specify how many
// times the proposed instruction could be executed. We removed this feature
// but we keep the minimal logic implemented in case it is re-added later.
// We removed it because it wasn't needed.
const defaultNumExecution uint64 = 1

// If ExpireBlockIndex is not given, we use this value plus the current block
// index to set the value of ExpireBlockIndex.
const defaultExpireThreshold uint64 = 50

// DeferredData contains the specific data of a deferred contract
type DeferredData struct {
	// The transaction that signers must sign and can be executed with an
	// "executeProposedTx".
	ProposedTransaction ClientTransaction
	// If the current block index is greater than this value, any Invoke on the
	// deferred contract is rejected. This provides an expiration mechanism.
	// This parameter is optional. If not given, it is set to
	// `current_blockIdx + defaultExpireThreshold`
	ExpireBlockIndex uint64
	// Hashes of each instruction of the proposed transaction. Those hashes are
	// computed using the special "hashDeferred" method.
	InstructionHashes [][]byte
	// The number of time the proposed transaction can be executed. This number
	// decreases for each successful invocation of "executeProposedTx" and its
	// default value is set to 1.
	MaxNumExecution uint64
	// This array is filled with the instruction IDs of each executed
	// instruction when a successful "executeProposedTx" happens.
	ExecResult [][]byte
}

// String returns a human readable string representation of the deferred data
func (dd DeferredData) String() string {
	out := new(strings.Builder)
	out.WriteString("- Proposed Tx:\n")
	for i, inst := range dd.ProposedTransaction.Instructions {
		fmt.Fprintf(out, "-- Instruction %d:\n", i)
		out.WriteString(eachLine.ReplaceAllString(inst.String(), "--$1"))
	}
	fmt.Fprintf(out, "- Expire Block Index: %d\n", dd.ExpireBlockIndex)
	fmt.Fprint(out, "- Instruction hashes: \n")
	for i, hash := range dd.InstructionHashes {
		fmt.Fprintf(out, "-- hash %d:\n", i)
		fmt.Fprintf(out, "--- %x\n", hash)
	}
	fmt.Fprintf(out, "- Max num execution: %d\n", dd.MaxNumExecution)
	fmt.Fprintf(out, "- Exec results: \n")
	for i, res := range dd.ExecResult {
		fmt.Fprintf(out, "-- res %d:\n", i)
		fmt.Fprintf(out, "--- %x\n", res)
	}
	return out.String()
}

type contractDeferred struct {
	BasicContract
	DeferredData
	contracts ReadOnlyContractRegistry
}

func contractDeferredFromBytes(in []byte) (Contract, error) {
	c := &contractDeferred{}

	err := protobuf.Decode(in, &c.DeferredData)
	if err != nil {
		return nil, errors.New("couldn't unmarshal instance data: " + err.Error())
	}
	return c, nil
}

// SetRegistry keeps the reference of the contract registry.
func (c *contractDeferred) SetRegistry(r ReadOnlyContractRegistry) {
	c.contracts = r
}

func (c *contractDeferred) Spawn(rst ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cout []Coin, err error) {
	// This method should do the following:
	//   1. Parse the input buffer
	//   2. Compute and store the instruction hashes
	//   3. Save the data
	//
	// Spawn should have those input arguments:
	//   - proposedTransaction ClientTransaction
	//   - expireBlockIndex uint64 (optional)
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	// 1. Reads and parses the input
	proposedTransaction := ClientTransaction{}
	err = protobuf.Decode(inst.Spawn.Args.Search("proposedTransaction"), &proposedTransaction)
	if err != nil {
		return nil, nil, errors.New("couldn't decode proposedTransaction: " + err.Error())
	}

	expireBlockIndexBuf := inst.Spawn.Args.Search("expireBlockIndex")
	var expireBlockIndex uint64
	if expireBlockIndexBuf == nil {
		expireBlockIndex = uint64(rst.GetIndex()) + defaultExpireThreshold
	} else {
		expireBlockIndex = binary.LittleEndian.Uint64(expireBlockIndexBuf)
		if err != nil {
			return nil, nil, errors.New("couldn't convert expireBlockIndex: " + err.Error())
		}
	}

	numExecution := defaultNumExecution

	// 2. Computes the hashes of each instruction and store it
	hash := make([][]byte, len(proposedTransaction.Instructions))
	for i, proposedInstruction := range proposedTransaction.Instructions {
		hash[i] = hashDeferred(proposedInstruction, inst.InstanceID.Slice())
	}

	// 3. Saves the data
	data := DeferredData{
		ProposedTransaction: proposedTransaction,
		ExpireBlockIndex:    expireBlockIndex,
		InstructionHashes:   hash,
		MaxNumExecution:     numExecution,
	}
	var dataBuf []byte
	dataBuf, err = protobuf.Encode(&data)
	if err != nil {
		return nil, nil, errors.New("couldn't encode DeferredData: " + err.Error())
	}

	sc = append(sc, NewStateChange(Create, inst.DeriveID(""),
		ContractDeferredID, dataBuf, darcID))
	return
}

func (c *contractDeferred) Invoke(rst ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cout []Coin, err error) {
	// This method should do the following:
	//   - Handle the "addProof" invocation
	//   - Handle the "execProposedTx" invocation
	//
	// Invoke:addProof should have the following input argument:
	//   - identity darc.Identity
	//   - signature []byte
	//   - index uint32 (index of the instruction wrt the transaction)
	err = c.checkInvoke(rst, inst.Invoke)
	if err != nil {
		return nil, nil, errors.New("checks of invoke failed: " + err.Error())
	}

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
		// which is based on the stored instruction hash (in instructionHashes)

		// Get the given index
		indexBuf := inst.Invoke.Args.Search("index")
		if indexBuf == nil {
			return nil, nil, errors.New("index args is nil")
		}
		index := binary.LittleEndian.Uint32(indexBuf)

		// Check if the index is in range
		numInstruction := len(c.DeferredData.ProposedTransaction.Instructions)
		if index >= uint32(numInstruction) {
			return nil, nil, fmt.Errorf("index is out of range (%d >= %d)", index, numInstruction)
		}

		// Get the given Identity
		identityBuf := inst.Invoke.Args.Search("identity")
		if identityBuf == nil {
			return nil, nil, errors.New("identity args is nil")
		}
		identity := darc.Identity{}
		err = protobuf.Decode(identityBuf, &identity)
		if err != nil {
			return nil, nil, errors.New("couldn't decode Identity")
		}

		// Get the given signature
		signature := inst.Invoke.Args.Search("signature")
		if signature == nil {
			return nil, nil, errors.New("signature args is nil")
		}
		// Update the contract's data with the given signature and identity
		c.DeferredData.ProposedTransaction.Instructions[index].SignerIdentities = append(c.DeferredData.ProposedTransaction.Instructions[index].SignerIdentities, identity)
		c.DeferredData.ProposedTransaction.Instructions[index].Signatures = append(c.DeferredData.ProposedTransaction.Instructions[index].Signatures, signature)
		// Save and send the modifications
		cosiDataBuf, err2 := protobuf.Encode(&c.DeferredData)
		if err2 != nil {
			return nil, nil, errors.New("couldn't encode DeferredData")
		}
		sc = append(sc, NewStateChange(Update, inst.InstanceID,
			ContractDeferredID, cosiDataBuf, darcID))
		return
	case "execProposedTx":
		// This invocation tries to execute the transaction stored with the
		// "Spawn" invocation. If it is successful, this invocation fills the
		// "ExecResult" field of the "deferredData" struct.
		// We couldn't successfully re-use one of the already implemented
		// method like the "processOneTx" one because it involved quite a lot
		// of changes and would bring more complexity compared to the benefits.

		// In the following we are creating a new StagingStateTrie from the
		// readonly state try by copying the data.
		nonce, err2 := rst.GetNonce()
		if err2 != nil {
			return nil, nil, errors.New("couldn't get the nonce: " + err2.Error())
		}
		sst, err2 := newMemStagingStateTrie(nonce)
		if err != nil {
			return nil, nil, errors.New("Failed to created stagingStateTrie: " + err2.Error())
		}
		err = rst.ForEach(sst.Set)
		if err != nil {
			return nil, nil, errors.New("couldn't make a copy of readOnlyStateTrie: " + err.Error())
		}

		instructionIDs := make([][]byte, len(c.DeferredData.ProposedTransaction.Instructions))

		for i, proposedInstr := range c.DeferredData.ProposedTransaction.Instructions {

			// In case it goes well, we want to return the proposed Tx InstanceID
			instructionIDs[i] = proposedInstr.DeriveID("").Slice()

			instructionType := proposedInstr.GetType()

			// Here we instantiate the contract from the state trie by getting
			// its buferred data and then calling its constructor.
			contractBuf, _, contractID, _, err := rst.GetValues(proposedInstr.InstanceID.Slice())
			if err != nil {
				return nil, nil, errors.New("couldn't get contract buf: " + err.Error())
			}
			// Get the contract's constructor (like "contractValueFromByte(...)")
			if c.contracts == nil {
				return nil, nil, errors.New("contracts registry is missing due to bad initialization")
			}

			fn, exists := c.contracts.Search(contractID)
			if !exists {
				return nil, nil, errors.New("couldn't get the root function")
			}
			// Invoke the contructor and get the contract's instance
			contract, err := fn(contractBuf)
			if err != nil {
				return nil, nil, errors.New("couldn't get the root contract: " + err.Error())
			}
			if cwr, ok := contract.(ContractWithRegistry); ok {
				cwr.SetRegistry(c.contracts)
			}

			err = contract.VerifyDeferredInstruction(sst, proposedInstr, c.DeferredData.InstructionHashes[i])
			if err != nil {
				return nil, nil, fmt.Errorf("verifying the instruction failed: %s", err)
			}

			var stateChanges []StateChange
			switch instructionType {
			case SpawnType:
				stateChanges, _, err = contract.Spawn(sst, proposedInstr, coins)
			case InvokeType:
				stateChanges, _, err = contract.Invoke(sst, proposedInstr, coins)
			case DeleteType:
				stateChanges, _, err = contract.Delete(sst, proposedInstr, coins)

			}

			if err != nil {
				return nil, nil, fmt.Errorf("error while executing an instruction: %s", err)
			}

			err = sst.StoreAll(stateChanges)
			if err != nil {
				return nil, nil, fmt.Errorf("error while storing state changes: %s", err)
			}

			sc = append(sc, stateChanges...)
		}

		c.DeferredData.ExecResult = instructionIDs
		// At this stage all verification passed. We can then decrease the
		// MaxNumExecution counter.
		c.DeferredData.MaxNumExecution = c.DeferredData.MaxNumExecution - 1
		resultBuf, err2 := protobuf.Encode(&c.DeferredData)
		if err2 != nil {
			return nil, nil, errors.New("couldn't encode the result")
		}
		sc = append(sc, NewStateChange(Update, inst.InstanceID,
			ContractDeferredID, resultBuf, darcID))

		return
	default:
		return nil, nil, errors.New("deferred contract can only addProof and execProposedTx")
	}
}

func (c *contractDeferred) Delete(rst ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cout []Coin, err error) {
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	sc = StateChanges{
		NewStateChange(Remove, inst.InstanceID, ContractDeferredID, nil, darcID),
	}
	return
}

func (c *contractDeferred) checkInvoke(rst ReadOnlyStateTrie, invoke *Invoke) error {

	// Global check on the invoke method:
	//   1. The MaxNumExecution should be greater than 0
	//   2. the current skipblock index should be lower than the provided
	//      "expireBlockIndex" argument.

	// 1.
	if c.DeferredData.MaxNumExecution < uint64(1) {
		return errors.New("maximum number of executions reached")
	}

	// 2.
	expireBlockIndex := c.DeferredData.ExpireBlockIndex
	currentIndex := uint64(rst.GetIndex())
	if currentIndex > expireBlockIndex {
		return fmt.Errorf("current block index is too high (%d > %d)", currentIndex, expireBlockIndex)
	}

	if invoke.Command == "addProof" {
		// We will go through 2 checks:
		//   1. Check if the identity is already stored
		//   2. Check if the signature is valid

		// 1:
		// Get the given Identity
		identityBuf := invoke.Args.Search("identity")
		if identityBuf == nil {
			return errors.New("identity args is nil")
		}
		identity := darc.Identity{}
		err := protobuf.Decode(identityBuf, &identity)
		if err != nil {
			return errors.New("couldn't decode Identity")
		}
		// Get the instruction index
		indexBuf := invoke.Args.Search("index")
		if indexBuf == nil {
			return errors.New("index args is nil")
		}
		index := binary.LittleEndian.Uint32(indexBuf)

		for _, storedIdentity := range c.DeferredData.ProposedTransaction.Instructions[index].SignerIdentities {
			if identity.Equal(&storedIdentity) {
				return errors.New("identity already stored")
			}
		}
		// 2:
		// Get the given signature
		signature := invoke.Args.Search("signature")
		if signature == nil {
			return errors.New("signature args is nil")
		}
		err = identity.Verify(c.InstructionHashes[index], signature)
		if err != nil {
			return errors.New("bad signature")
		}
	}
	return nil
}

func (c *contractDeferred) VerifyInstruction(rst ReadOnlyStateTrie, inst Instruction, ctxHash []byte) error {
	// We make a special case for the delete instruction. Anyone should be able
	// to delete a deferred contract that has expired.
	if inst.GetType() == DeleteType && uint64(rst.GetIndex()) >= c.DeferredData.ExpireBlockIndex {
		return nil
	}
	if err := inst.Verify(rst, ctxHash); err != nil {
		return err
	}
	return nil
}

// This is a modified version of computing the hash of a transaction. In this
// version, we do not take into account the signers nor the signers counters. We
// also add to the hash the instanceID of the deferred contract.
func hashDeferred(instr Instruction, instanceID []byte) []byte {
	h := sha256.New()
	h.Write(instr.InstanceID[:])
	var args []Argument
	switch instr.GetType() {
	case SpawnType:
		h.Write([]byte{0})
		h.Write([]byte(instr.Spawn.ContractID))
		args = instr.Spawn.Args
	case InvokeType:
		h.Write([]byte{1})
		h.Write([]byte(instr.Invoke.ContractID))
		args = instr.Invoke.Args
	case DeleteType:
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
	h.Write(instanceID)

	return h.Sum(nil)
}
