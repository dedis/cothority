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

func NewTransaction(data []byte) *Transaction {
	_, blob, _ := network.Unmarshal(data, cothority.Suite)
	transaction, _ := blob.(*Transaction)
	return transaction
}
