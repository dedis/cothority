package lib

import (
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority"
)

func init() {
	network.RegisterMessage(&Transaction{})
}

type Transaction struct {
	Election *Election
	Ballot   *Ballot
	Mix      *Mix
	Partial  *Partial

	User      uint32
	Signature []byte
}

func UnmarshalTransaction(data []byte) *Transaction {
	_, blob, _ := network.Unmarshal(data, cothority.Suite)
	transaction, _ := blob.(*Transaction)
	return transaction
}

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
