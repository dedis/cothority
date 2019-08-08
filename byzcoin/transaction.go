package byzcoin

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"regexp"
	"strings"
	"sync"

	"go.dedis.ch/cothority/v3/byzcoin/trie"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// An InstanceID is a unique identifier for one instance of a contract.
type InstanceID [32]byte

// eachLine matches the content of non-empty lines
var eachLine = regexp.MustCompile(`(?m)^(.+)$`)

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

// Names returns a slice of the names of the arguments.
func (args Arguments) Names() []string {
	var names []string
	for _, arg := range args {
		names = append(names, arg.Name)
	}
	return names
}

// FillSignersAndSignWith fills the SignerIdentities field with the identities of the signers and then signs all the
// instructions using the same set of  signers. If some instructions need to be signed by different sets of signers,
// then use the SignWith method of Instruction.
func (ctx *ClientTransaction) FillSignersAndSignWith(signers ...darc.Signer) error {
	var ids []darc.Identity
	for _, signer := range signers {
		ids = append(ids, signer.Identity())
	}
	for i := range ctx.Instructions {
		ctx.Instructions[i].SignerIdentities = ids
	}
	return ctx.SignWith(signers...)
}

// SignWith signs all the instructions with the same signers. If some instructions need to be signed by different sets
// of signers, then use the SignWith method of Instruction.
func (ctx *ClientTransaction) SignWith(signers ...darc.Signer) error {
	digest := ctx.Instructions.Hash()
	for i := range ctx.Instructions {
		if err := ctx.Instructions[i].SignWith(digest, signers...); err != nil {
			return err
		}
	}
	return nil
}

// NewClientTransaction creates a transaction compatible with the version passed
// in arguments. Depending on the version, the hash will have a different value.
func NewClientTransaction(v Version, instrs ...Instruction) ClientTransaction {
	ctx := ClientTransaction{Instructions: instrs}
	ctx.Instructions.SetVersion(v)

	return ctx
}

// Hash computes the digest of the hash function
func (instr Instruction) Hash() []byte {
	h := sha256.New()
	instr.hashType(h)
	instr.hashSigners(h)
	return h.Sum(nil)
}

func (instr Instruction) hashType(h hash.Hash) {
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
		if instr.version >= 1 {
			h.Write([]byte(instr.Invoke.Command))
		}
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
}

func (instr Instruction) hashSigners(h hash.Hash) {
	for _, ctr := range instr.SignerCounter {
		ctrBuf := make([]byte, 8)
		binary.LittleEndian.PutUint64(ctrBuf, ctr)
		h.Write(ctrBuf)
	}
	for _, id := range instr.SignerIdentities {
		buf := id.GetPublicBytes()
		lenBuf := make([]byte, 8)
		binary.LittleEndian.PutUint64(lenBuf, uint64(len(buf)))
		h.Write(lenBuf)
		h.Write(buf)
	}
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

	for _, sig := range instr.Signatures {
		binary.LittleEndian.PutUint32(b[:], uint32(len(sig)))
		h.Write(b[:])
		h.Write(sig)
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
	contractID := instr.ContractID()
	a := "invalid"
	switch instr.GetType() {
	case SpawnType:
		a = "spawn:" + contractID
	case InvokeType:
		a = "invoke:" + contractID + "." + instr.Invoke.Command
	case DeleteType:
		a = "delete:" + contractID
	}
	return a
}

// ContractID returns the ContractID that the user specified
func (instr Instruction) ContractID() string {
	a := "invalid"
	switch instr.GetType() {
	case SpawnType:
		a = instr.Spawn.ContractID
	case InvokeType:
		a = instr.Invoke.ContractID
	case DeleteType:
		a = instr.Delete.ContractID
	}
	return a
}

// String returns a human readable form of the instruction.
func (instr Instruction) String() string {
	contractFn, ok := GetContractRegistry().Search(instr.ContractID())
	var methodStr string
	if !ok {
		methodStr = "error in getting constructor: " + instr.ContractID()
	} else {
		contract, err := contractFn(nil)
		if err != nil {
			methodStr = "error in getting contract"
		} else {
			methodStr = contract.FormatMethod(instr)
		}
	}

	var out strings.Builder
	out.WriteString("- instruction:\n")
	fmt.Fprintf(&out, "-- hash: %x\n", instr.Hash())
	fmt.Fprintf(&out, "-- instID: %v\n", instr.InstanceID)
	fmt.Fprintf(&out, "-- action: %s\n", instr.Action())
	fmt.Fprintf(&out, "-- identities: %v\n", instr.SignerIdentities)
	fmt.Fprintf(&out, "-- counters: %v\n", instr.SignerCounter)
	fmt.Fprintf(&out, "-- signatures: %d\n", len(instr.Signatures))
	out.WriteString(eachLine.ReplaceAllString(methodStr, "-$1"))

	return out.String()
}

// SignWith creates a signed version of the instruction. The signature is
// created on msg, which must be the hash of the ClientTransaction which
// contains the instruction. Otherwise the verification will fail on the server
// side.
func (instr *Instruction) SignWith(msg []byte, signers ...darc.Signer) error {
	if len(signers) != len(instr.SignerIdentities) {
		return errors.New("the number of signers does not match the number of identities")
	}
	if len(signers) != len(instr.SignerCounter) {
		return errors.New("the number of signers does not match the number of counters")
	}
	instr.Signatures = make([][]byte, len(signers))
	for i := range signers {
		signerID := signers[i].Identity()
		if !instr.SignerIdentities[i].Equal(&signerID) {
			return errors.New("signer identity is not set correctly")
		}
		sig, err := signers[i].Sign(msg)
		if err != nil {
			return err
		}
		instr.Signatures[i] = sig
	}
	return nil
}

// GetIdentityStrings gets a slice of identities who are signing the
// instruction.
func (instr Instruction) GetIdentityStrings() []string {
	res := make([]string, len(instr.SignerIdentities))
	for i, id := range instr.SignerIdentities {
		res[i] = id.String()
	}
	return res
}

// VerificationOptions is given to VerifyWithOption if the verification needs
// to be customized.
type VerificationOptions struct {
	IgnoreCounters bool
	EvalAttr       darc.AttrInterpreters
}

// Verify will look up the darc of the instance pointed to by the instruction
// and then verify if the signature on the instruction can satisfy the rules of
// the darc. An error is returned if any of the verification fails.
func (instr Instruction) Verify(st ReadOnlyStateTrie, msg []byte) error {
	return instr.VerifyWithOption(st, msg, nil)
}

// VerifyWithOption adds the ability to the Verify(...) method to specify if
// the counters should be checked. This is used with the "defered" contract
// where the clients sign the root instruction without the counters.
func (instr Instruction) VerifyWithOption(st ReadOnlyStateTrie, msg []byte, ops *VerificationOptions) error {
	if ops == nil {
		ops = &VerificationOptions{}
	}

	// check the number of signers match with the number of signatures
	if len(instr.SignerIdentities) != len(instr.Signatures) {
		return errors.New("lengh of identities does not match the length of signatures")
	}

	// check the signature counters
	if !ops.IgnoreCounters {
		if err := verifySignerCounters(st, instr.SignerCounter, instr.SignerIdentities); err != nil {
			return err
		}
	}

	// get the valid DARC contract IDs from the configuration
	config, err := LoadConfigFromTrie(st)
	if err != nil {
		return err
	}

	// get the darc
	d, err := getInstanceDarc(st, instr.InstanceID, config.DarcContractIDs)
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
	// Save the identities that provide good signatures
	goodIdentities := make([]string, 0)
	for i := range instr.Signatures {
		if err := instr.SignerIdentities[i].Verify(msg, instr.Signatures[i]); err == nil {
			goodIdentities = append(goodIdentities, instr.SignerIdentities[i].String())
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
		d, err := LoadDarcFromTrie(st, darcID)
		if err != nil {
			return nil
		}
		return d
	}

	if ops.EvalAttr != nil {
		return darc.EvalExprAttr(d.Rules.Get(darc.Action(instr.Action())), getDarc, ops.EvalAttr, goodIdentities...)
	}
	return darc.EvalExpr(d.Rules.Get(darc.Action(instr.Action())), getDarc, goodIdentities...)
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

// HashWithSignatures calculates the hash over all instructions and their signatures.
// This creates a unique hash with regard to two sets of instructions
// that only differ with their signature.
func (instrs Instructions) HashWithSignatures() []byte {
	h := sha256.New()
	for _, inst := range instrs {
		h.Write(inst.Hash())
		for _, sig := range inst.Signatures {
			h.Write(sig)
		}
	}
	return h.Sum(nil)
}

// SetVersion makes sure the underlying data will use the implementation
// of the given version.
func (instrs Instructions) SetVersion(version Version) {
	for i := range instrs {
		instrs[i].version = version
	}
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

// SetVersion makes sure the underlying data will use the implementation
// of the given version.
func (txr TxResults) SetVersion(version Version) {
	for _, tx := range txr {
		tx.ClientTransaction.Instructions.SetVersion(version)
	}
}

// NewStateChange is a convenience function that fills out a StateChange
// structure.
func NewStateChange(sa StateAction, iID InstanceID, contractID string, value []byte, darcID darc.ID) StateChange {
	return StateChange{
		StateAction: sa,
		InstanceID:  append([]byte{}, iID[:]...),
		ContractID:  contractID,
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

const defaultMaxBufferSize = 1000

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

func (r *txBuffer) take(key string, max int) []ClientTransaction {
	r.Lock()
	defer r.Unlock()

	txs, ok := r.txsMap[key]
	if !ok {
		return []ClientTransaction{}
	}

	out := len(txs)
	if max >= 0 && out > max {
		// Take only up to the maximum required transactions.
		out = max
	}

	ret := make([]ClientTransaction, out)
	copy(ret, txs)

	if out == max {
		// Keep the overflow for the next collection.
		r.txsMap[key] = txs[max:]
	} else {
		delete(r.txsMap, key)
	}

	return ret
}

func (r *txBuffer) add(key string, newTx ClientTransaction) {
	r.Lock()
	defer r.Unlock()

	if txs, ok := r.txsMap[key]; !ok {
		r.txsMap[key] = []ClientTransaction{newTx}
	} else {
		if len(txs) >= defaultMaxBufferSize {
			// Drop transactions if the buffer is full. We cannot drop earlier
			// transactions because an attacker could send multiple ones to
			// replace legit transactions.
			return
		}

		txs = append(txs, newTx)
		r.txsMap[key] = txs
	}
}
