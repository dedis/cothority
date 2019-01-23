package byzcoin

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/dedis/cothority/byzcoin/trie"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// An InstanceID is a unique identifier for one instance of a contract.
type InstanceID [32]byte

func (iID InstanceID) String() string {
	return fmt.Sprintf("%x", iID.Slice())
}

// Nonce is used to prevent replay attacks in instructions.
type Nonce [32]byte

func init() {
	network.RegisterMessages(Instruction{}, TxResult{},
		StateChange{})
}

// NewNonce returns a nonce given a slice of bytes.
func NewNonce(buf []byte) Nonce {
	if len(buf) != 32 {
		return Nonce{}
	}
	n := Nonce{}
	copy(n[:], buf)
	return n
}

// NewInstanceID converts the first 32 bytes of in into an InstanceID.
// Giving nil as in results in the zero InstanceID, which is the special
// key that holds the ledger config.
func NewInstanceID(in []byte) InstanceID {
	var i InstanceID
	copy(i[:], in)
	return i
}

// Equal returns if both InstanceIDs point to the same instance.
func (iID InstanceID) Equal(other InstanceID) bool {
	return bytes.Equal(iID[:], other[:])
}

// Slice returns the InstanceID as a []byte.
func (iID InstanceID) Slice() []byte {
	return iID[:]
}

// Arguments is a searchable list of arguments.
type Arguments []Argument

// Search returns the value of a given argument. If it is not found, nil
// is returned.
// TODO: An argument with nil value cannot be distinguished from
// a missing argument!
func (args Arguments) Search(name string) []byte {
	for _, arg := range args {
		if arg.Name == name {
			return arg.Value
		}
	}
	return nil
}

// SignWith signs all the instructions in the transaction using the same set of
// signers. If some instructions need to be signed by different sets of
// signers, then use the SighWith method of Instruction.
func (ctx *ClientTransaction) SignWith(signers ...darc.Signer) error {
	h := ctx.Instructions.Hash()
	for i := range ctx.Instructions {
		if err := ctx.Instructions[i].SignWith(h, signers...); err != nil {
			return err
		}
	}
	return nil
}

// Hash computes the digest of the hash function
func (instr Instruction) Hash() []byte {
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
		h.Write([]byte(a.Name))
		h.Write(a.Value)
	}
	for _, ver := range instr.SignerCounter {
		verBuf := make([]byte, 8)
		binary.LittleEndian.PutUint64(verBuf, ver)
		h.Write(verBuf)
	}
	return h.Sum(nil)
}

// DeriveID derives a new InstanceID from the hash of the instruction, its signatures,
// and the given string.
//
// DeriveID is used inside of contracts that need to create additional keys in
// the trie. By convention newly spawned instances should have their
// InstanceID derived via inst.DeriveID("").
func (instr Instruction) DeriveID(what string) InstanceID {
	var b [4]byte

	// Une petit primer on domain separation in hashing:
	// Domain separation is required when the input has variable lengths,
	// because an attacker could try to construct two messages resulting in the
	// same hash by moving bytes from one neighboring input to the
	// other. With fixed-length inputs, moving bytes is not possible, so
	// no domain separation is needed.

	h := sha256.New()
	h.Write(instr.Hash())

	binary.LittleEndian.PutUint32(b[:], uint32(len(instr.Signatures)))
	h.Write(b[:])

	for _, s := range instr.Signatures {
		binary.LittleEndian.PutUint32(b[:], uint32(len(s.Signature)))
		h.Write(b[:])
		h.Write(s.Signature)
		// TODO: Why not h.Write(s.Signer)
	}
	// Because there is no attacker-controlled input after what, we do not need
	// domain separation here.
	h.Write([]byte(what))

	return NewInstanceID(h.Sum(nil))

	// Addendum:
	//
	// While considering this we also considered the possibility that
	// allowing the attackers to mess with the signatures in order to
	// attempt to create InstanceID collisions is not a risk, since moving
	// a byte from sig[1] over to sig[0] would invalidate both signatures.
	// This is true for the Schnorr sigs we use today, but if there's some
	// other kind of data in the Signature field in the future, it might
	// be tolerant of mutations, meaning that what seems unrisky today could
	// be leaving a trap for later. So to be conservative, we are implementing
	// strict domain separation now.
}

// Action returns the action that the user wants to do with this
// instruction.
func (instr Instruction) Action() string {
	a := "invalid"
	switch instr.GetType() {
	case SpawnType:
		a = "spawn:" + instr.Spawn.ContractID
	case InvokeType:
		a = "invoke:" + instr.Invoke.ContractID + "." + instr.Invoke.Command
	case DeleteType:
		a = "delete:" + instr.Delete.ContractID
	}
	return a
}

// String returns a human readable form of the instruction.
func (instr Instruction) String() string {
	var out string
	out += fmt.Sprintf("instr: %x\n", instr.Hash())
	out += fmt.Sprintf("\tinstID: %v\n", instr.InstanceID)
	out += fmt.Sprintf("\taction: %s\n", instr.Action())
	out += fmt.Sprintf("\tcounters: %v\n", instr.SignerCounter)
	out += fmt.Sprintf("\tsignatures: %d\n", len(instr.Signatures))
	return out
}

// SignWith creates a signed version of the instruction. The signature is
// created on msg, which must be the hash of the ClientTransaction which
// contains the instruction. Otherwise the verification will fail on the server
// side.
func (instr *Instruction) SignWith(msg []byte, signers ...darc.Signer) error {
	instr.Signatures = make([]darc.Signature, len(signers))
	for i := range signers {
		sig, err := signers[i].Sign(msg)
		if err != nil {
			return err
		}
		instr.Signatures[i] = darc.Signature{
			Signature: sig,
			Signer:    signers[i].Identity(),
		}
	}
	return nil
}

// GetIdentityStrings gets a slice of identities who are signing the
// instruction.
func (instr Instruction) GetIdentityStrings() []string {
	res := make([]string, len(instr.Signatures))
	for i, sig := range instr.Signatures {
		res[i] = sig.Signer.String()
	}
	return res
}

// Verify will look up the darc of the instance pointed to by the instruction
// and then verify if the signature on the instruction can satisfy the rules of
// the darc. An error is returned if any of the verification fails.
func (instr Instruction) Verify(st ReadOnlyStateTrie, msg []byte) error {
	// check the signature counters
	if err := verifySignerCounters(st, instr.SignerCounter, instr.Signatures); err != nil {
		return err
	}

	// get the darc
	d, err := getInstanceDarc(st, instr.InstanceID)
	if err != nil {
		return errors.New("darc not found: " + err.Error())
	}
	if len(instr.Signatures) == 0 {
		return errors.New("no signatures - nothing to verify")
	}

	// check the action
	if !d.Rules.Contains(darc.Action(instr.Action())) {
		return fmt.Errorf("action '%v' does not exist", instr.Action())
	}

	// check the signature
	for _, sig := range instr.Signatures {
		if err := sig.Signer.Verify(msg, sig.Signature); err != nil {
			return err
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
		d, err := loadDarcFromTrie(st, darcID)
		if err != nil {
			return nil
		}
		return d
	}
	return darc.EvalExpr(d.Rules.Get(darc.Action(instr.Action())), getDarc, instr.GetIdentityStrings()...)
}

// InstrType is the instruction type, which can be spawn, invoke or delete.
type InstrType int

const (
	// InvalidInstrType represents an error in the instruction type.
	InvalidInstrType InstrType = iota
	// SpawnType represents the spawn instruction type.
	SpawnType
	// InvokeType represents the invoke instruction type.
	InvokeType
	// DeleteType represents the delete instruction type.
	DeleteType
)

// GetType returns the type of the instruction.
func (instr Instruction) GetType() InstrType {
	if instr.Spawn != nil && instr.Invoke == nil && instr.Delete == nil {
		return SpawnType
	} else if instr.Spawn == nil && instr.Invoke != nil && instr.Delete == nil {
		return InvokeType
	} else if instr.Spawn == nil && instr.Invoke == nil && instr.Delete != nil {
		return DeleteType
	}
	return InvalidInstrType
}

// Instructions is a slice of Instruction
type Instructions []Instruction

// Hash returns the sha256 hash of the hash of every instruction.
func (instrs Instructions) Hash() []byte {
	h := sha256.New()
	for _, instr := range instrs {
		h.Write(instr.Hash())
	}
	return h.Sum(nil)
}

// TxResults is a list of results from executed transactions.
type TxResults []TxResult

// NewTxResults takes a list of client transactions and wraps them up
// in a TxResults with Accepted set to false for each.
func NewTxResults(ct ...ClientTransaction) TxResults {
	out := make([]TxResult, len(ct))
	for i := range ct {
		out[i].ClientTransaction = ct[i]
	}
	return out
}

// Hash returns the sha256 hash of all of the transactions.
func (txr TxResults) Hash() []byte {
	one := []byte{1}
	zero := []byte{0}

	h := sha256.New()
	for _, tx := range txr {
		h.Write(tx.ClientTransaction.Instructions.Hash())
		if tx.Accepted {
			h.Write(one[:])
		} else {
			h.Write(zero[:])
		}
	}
	return h.Sum(nil)
}

// NewStateChange is a convenience function that fills out a StateChange
// structure.
func NewStateChange(sa StateAction, iID InstanceID, contractID string, value []byte, darcID darc.ID) StateChange {
	return StateChange{
		StateAction: sa,
		InstanceID:  append([]byte{}, iID[:]...),
		ContractID:  append([]byte{}, []byte(contractID)...),
		Value:       value,
		DarcID:      darcID,
	}
}

func (sc StateChange) toString(withValue bool) string {
	var out string
	out += "\nstatechange\n"
	out += fmt.Sprintf("\taction: %s\n", sc.StateAction)
	out += fmt.Sprintf("\tcontractID: %s\n", string(sc.ContractID))
	out += fmt.Sprintf("\tkey: %x\n", sc.InstanceID)
	out += fmt.Sprintf("\tversion: %d\n", sc.Version)
	if withValue {
		out += fmt.Sprintf("\tvalue: %x", sc.Value)
	}
	return out
}

// String can be used in print.
func (sc StateChange) String() string {
	return sc.toString(true)
}

// ShortString is the same as String but excludes the value part.
func (sc StateChange) ShortString() string {
	return sc.toString(false)
}

// Key returns the key that should be used in a key/value database.
func (sc *StateChange) Key() []byte {
	return sc.InstanceID
}

// Val returns the value that should be used in a key/value database.
func (sc *StateChange) Val() []byte {
	v := StateChangeBody{
		StateAction: sc.StateAction,
		ContractID:  sc.ContractID,
		Value:       sc.Value,
		Version:     sc.Version,
		DarcID:      sc.DarcID,
	}
	buf, err := protobuf.Encode(&v)
	if err != nil {
		log.Error("failed to encode statechange value")
		return nil
	}
	return buf
}

// Op returns the operation type of the state change, which is either a set or
// a delete.
func (sc *StateChange) Op() trie.OpType {
	switch sc.StateAction {
	case Create, Update:
		return trie.OpSet
	case Remove:
		return trie.OpDel
	}
	return 0
}

func decodeStateChangeBody(buf []byte) (StateChangeBody, error) {
	var out StateChangeBody
	err := protobuf.Decode(buf, &out)
	return out, err
}

// StateChanges hold a slice of StateChange
type StateChanges []StateChange

// Hash returns the sha256 of all stateChanges
func (scs StateChanges) Hash() []byte {
	h := sha256.New()
	for _, sc := range scs {
		scBuf, err := protobuf.Encode(&sc)
		if err != nil {
			log.Lvl2("Couldn't marshal transaction")
		}
		h.Write(scBuf)
	}
	return h.Sum(nil)
}

// ShortStrings outputs the ShortString of every state change.
func (scs StateChanges) ShortStrings() []string {
	out := make([]string, len(scs))
	for i, sc := range scs {
		out[i] = sc.ShortString()
	}
	return out
}

// StateAction describes how the trie will be modified.
type StateAction int

const (
	// Create allows to insert a new key-value association.
	Create StateAction = iota + 1
	// Update allows to change the value of an existing key.
	Update
	// Remove allows to delete an existing key-value association.
	Remove
)

// String returns a readable output of the action.
func (sc StateAction) String() string {
	switch sc {
	case Create:
		return "Create"
	case Update:
		return "Update"
	case Remove:
		return "Remove"
	default:
		return "Invalid stateChange"
	}
}

// txBuffer is thread-safe data structure that store client transactions.
type txBuffer struct {
	sync.Mutex
	txsMap map[string][]ClientTransaction
}

func newTxBuffer() txBuffer {
	return txBuffer{
		txsMap: make(map[string][]ClientTransaction),
	}
}

func (r *txBuffer) take(key string) []ClientTransaction {
	r.Lock()
	defer r.Unlock()

	txs, ok := r.txsMap[key]
	if !ok {
		return []ClientTransaction{}
	}
	delete(r.txsMap, key)
	return txs
}

func (r *txBuffer) add(key string, newTx ClientTransaction) {
	r.Lock()
	defer r.Unlock()

	if txs, ok := r.txsMap[key]; !ok {
		r.txsMap[key] = []ClientTransaction{newTx}
	} else {
		txs = append(txs, newTx)
		r.txsMap[key] = txs
	}
}
