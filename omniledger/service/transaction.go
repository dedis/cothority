package service

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/dedis/student_18_omniledger/omniledger/collection"
	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"gopkg.in/dedis/onet.v2/network"
)

func init() {
	network.RegisterMessages(Instruction{}, ClientTransaction{},
		OmniledgerTransaction{}, StateChange{})
}

// Instruction is created by a client. It has the following format:
type Instruction struct {
	// DarcID points to the darc that can verify the signature
	DarcID darc.ID
	// Nonce: will be concatenated to the darcID to create the key
	Nonce []byte
	// Command is object-specific and case-sensitive. The only command common to
	// all classes is "Create".
	Command string
	// Kind is only used when the command is "Create"
	Kind string `proto:"opt"`
	// Data is a free slice of bytes that can be sent to the object.
	Data []byte
	// Signatures that can be verified using the given darcID
	Signatures []darc.Signature
}

// Hash computes the digest of the hash function
func (instr Instruction) Hash() []byte {
	h := sha256.New()
	h.Write(instr.DarcID)
	h.Write(instr.Nonce)
	h.Write([]byte(instr.Command))
	h.Write([]byte(instr.Kind))
	h.Write(instr.Data)
	return h.Sum(nil)
}

// ToDarcRequest converts the Transaction content into a darc.Request.
func (instr Instruction) ToDarcRequest(kind string) (*darc.Request, error) {
	if len(instr.DarcID) < darcIDLen {
		return nil, errors.New("incorrect transaction length")
	}
	baseID := instr.DarcID
	action := kind
	ids := make([]*darc.Identity, len(instr.Signatures))
	sigs := make([][]byte, len(instr.Signatures))
	for i, sig := range instr.Signatures {
		ids[i] = &sig.Signer
		sigs[i] = sig.Signature // TODO shallow copy is ok?
	}
	req := darc.InitRequest(baseID, darc.Action(action), instr.Hash(), ids, sigs)
	return &req, nil
}

// GetKindState searches for the kind of this instruction. It needs the collection
// to do so.
func (instr Instruction) GetKindState(coll collection.Collection) (kind string, state []byte, err error) {
	// Getting the kind is different for instructions that create a key
	// and for instructions that send a call to an existing key.
	if instr.Command == "Create" {
		// For new key creations it is easy, as the first call needs to be
		// "Create" with the kind given in the data.
		return instr.Kind, nil, nil
	}

	// For existing keys, we need to go look the kind up in our database
	// to find the kind.
	kv := coll.Get(instr.GetKey())
	var record collection.Record
	record, err = kv.Record()
	if err != nil {
		return
	}
	var kindValue []interface{}
	kindValue, err = record.Values()
	if err != nil {
		return
	}
	kind = string(kindValue[0].([]byte))
	state = kindValue[1].([]byte)
	return
}

// GetKey returns the DarcID concatenated with the nonce.
func (instr Instruction) GetKey() []byte {
	return append(instr.DarcID, instr.Nonce...)
}

// ClientTransaction is a slice of Instructions that will be applied in order.
// If any of the instructions fails, none of them will be applied.
type ClientTransaction struct {
	Instructions []Instruction
}

// StateChange is one new state that will be applied to the collection.
type StateChange struct {
	// Action can be any of Create, Update, Remove
	Action Action
	// Key is the darcID concatenated with the Nonce
	Key []byte
	// Kind points to the class that can interpret the value
	Kind []byte
	// Value is the data needed by the class
	Value []byte
	// Previous points to the blockID of the previous StateChange for this Key
	Previous []byte
}

// NewStateChange is a convenience function that fills out a StateChange
// structure. For the moment it ignores previous. If a nil-nonce is given,
// it will be filled with 32 0 bytes.
func NewStateChange(a Action, darcid, nonce []byte, kind string, value []byte) StateChange {
	if nonce == nil {
		nonce = make([]byte, 32)
	}
	return StateChange{
		Action: a,
		Key:    append(darcid, nonce...),
		Kind:   []byte(kind),
		Value:  value,
	}
}

// String can be used in print.
func (sc StateChange) String() string {
	return fmt.Sprintf("%s(%s): %x / %x", sc.Action, sc.Kind, sc.Key, sc.Value)
}

// Action describes how the collectionDB will be modified.
type Action int

const (
	// Create allows to insert a new key-value association.
	Create Action = iota + 1
	// Update allows to change the value of an existing key.
	Update
	// Remove allows to delete an existing key-value association.
	Remove
)

// String returns a readable output of the action.
func (a Action) String() string {
	switch a {
	case Create:
		return "create"
	case Update:
		return "update"
	case Remove:
		return "remove"
	default:
		return "invalid action"
	}
}

// OmniledgerTransaction combines ClientTransactions and their StateChange. Plus
// it indicates if the leader thinks this transaction is valid or not.
type OmniledgerTransaction struct {
	// ClientTransaction is one transaction from the client.
	ClientTransaction ClientTransaction
	// StateChanges are the resulting changes in the global state
	StateChanges []StateChange
	// Valid is set by the leader
	Valid bool
}
