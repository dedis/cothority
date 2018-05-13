package service

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/onet.v2/network"

	"github.com/dedis/student_18_omniledger/omniledger/collection"
	"github.com/dedis/student_18_omniledger/omniledger/darc"
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

// SignBy gets signers to sign the (receiver) transaction.
func (instr *Instruction) SignBy(signers ...*darc.Signer) error {
	// Create the request and populate it with the right identities.  We
	// need to do this prior to signing because identities are a part of
	// the digest.
	req, err := instr.ToDarcRequest()
	if err != nil {
		return err
	}
	req.Identities = make([]*darc.Identity, len(signers))
	for i := range signers {
		req.Identities[i] = signers[i].Identity()
	}

	// Sign the instruction and write the signatures to it.
	digest, err := req.Hash()
	if err != nil {
		return err
	}
	instr.Signatures = make([]darc.Signature, len(signers))
	for i := range signers {
		sig, err := signers[i].Sign(digest)
		if err != nil {
			return err
		}
		instr.Signatures[i] = darc.Signature{
			Signature: sig,
			Signer:    *signers[i].Identity(),
		}
	}
	return nil
}

// ToDarcRequest converts the Instruction content into a darc.Request.
func (instr Instruction) ToDarcRequest() (*darc.Request, error) {
	baseID := instr.DarcID
	action := instr.Command
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

func (instr Instruction) String() string {
	var out string
	out += fmt.Sprintf("instr: %x\n", instr.Hash())
	out += fmt.Sprintf("\tcommand: %s\n", instr.Command)
	out += fmt.Sprintf("\tkind: %s\n", instr.Kind)
	out += fmt.Sprintf("\tdarc ID: %x\n", instr.DarcID)
	out += fmt.Sprintf("\tnonce: %x\n", instr.Nonce)
	out += fmt.Sprintf("\tsignatures: %d", len(instr.Signatures))
	return out
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
	var out string
	out += "statechange"
	out += fmt.Sprintf("\taction: %s\n", sc.Action)
	out += fmt.Sprintf("\tkind: %s\n", sc.Kind)
	out += fmt.Sprintf("\tkey: %x\n", sc.Key)
	out += fmt.Sprintf("\tvalue: %x", sc.Value)
	return out
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

// sortWithSalt sorts transactions according to their salted hash:
// The salt is prepended to the transactions []byte representation
// and this concatenation is hashed then.
// Using a salt here makes the resulting order of the transactions
// harder to guess.
func sortWithSalt(ts [][]byte, salt []byte) {
	less := func(i, j int) bool {
		h1 := sha256.Sum256(append(salt, ts[i]...))
		h2 := sha256.Sum256(append(salt, ts[j]...))
		return bytes.Compare(h1[:], h2[:]) == -1
	}
	sort.Slice(ts, less)
}

// sortTransactions needs to marshal transactions, if it fails to do so,
// it returns an error and leaves the slice unchange.
// The helper functions (sortWithSalt, xorTransactions) operate on []byte
// representations directly. This allows for some more compact error handling
// when (un)marshalling.
func sortTransactions(ts []ClientTransaction) error {
	bs := make([][]byte, len(ts))
	sortedTs := make([]*ClientTransaction, len(ts))
	var err error
	var ok bool
	for i := range ts {
		bs[i], err = network.Marshal(&ts[i])
		if err != nil {
			return err
		}
	}
	// An alternative to XOR-ing the transactions would have been to
	// concatenate them and hash the result. However, if we generate the salt
	// as the hash of the concatenation of the transactions, we have to
	// concatenate them in a specific order to be deterministic.
	// This means we would have to sort them, just to get the salt.
	// In order to avoid this, we XOR them.
	salt := xorTransactions(bs)
	sortWithSalt(bs, salt)
	for i := range bs {
		_, tmp, err := network.Unmarshal(bs[i], cothority.Suite)
		if err != nil {
			return err
		}
		sortedTs[i], ok = tmp.(*ClientTransaction)
		if !ok {
			return errors.New("Data of wrong type")
		}
	}
	for i := range sortedTs {
		ts[i] = *sortedTs[i]
	}
	return nil
}

// xorTransactions returns the XOR of the hash values of all the transactions.
func xorTransactions(ts [][]byte) []byte {
	result := make([]byte, sha256.Size)
	for _, t := range ts {
		hs := sha256.Sum256(t)
		for i := range result {
			result[i] = result[i] ^ hs[i]
		}
	}
	return result
}
