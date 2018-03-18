package lib

import (
	"github.com/dedis/onet/network"

	uuid "github.com/satori/go.uuid"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
)

func init() {
	network.RegisterMessage(&Transaction{})
}

// TransactionVerifierID identifes the core transaction verification function.
var TransactionVerifierID = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, ""))

// TransactionVerifiers is a list of accepted skipchain verification functions.
var TransactionVerifiers = []skipchain.VerifierID{TransactionVerifierID}

// Transaction is the sole data structure withing the blocks of an election
// skipchain, it holds all the other containers.
type Transaction struct {
	Election *Election
	Ballot   *Ballot
	Mix      *Mix
	Partial  *Partial

	User      uint32
	Signature []byte
}

// UnmarshalTransaction decodes a data blob to a transaction structure.
func UnmarshalTransaction(data []byte) *Transaction {
	_, blob, _ := network.Unmarshal(data, cothority.Suite)
	transaction, _ := blob.(*Transaction)
	return transaction
}

// NewTransaction constructs a new transaction for the given arguments.
func NewTransaction(data interface{}, user uint32, signature []byte) *Transaction {
	transaction := &Transaction{User: user, Signature: signature}
	switch data.(type) {
	case *Election:
		transaction.Election = data.(*Election)
	case *Ballot:
		transaction.Ballot = data.(*Ballot)
	case *Mix:
		transaction.Mix = data.(*Mix)
	case *Partial:
		transaction.Partial = data.(*Partial)
	default:
		return nil
	}
	return transaction
}
