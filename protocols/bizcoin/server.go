package bizcoin

import (
	"sync"

	"github.com/dedis/cothority/protocols/bizcoin/blockchain/blkparser"
)

// Server is the longterm control service that listens for transactions and
// dispatch them to a new BizCoin for each new signing that we want to do.
// It creates the BizCoin protocols and run them. only used by the root since
// only the root pariticipates to the creation of the block.
type Server struct {
	// transaction pool where all the incoming transactions are stored
	transactions []blkparser.Tx
	// lock associated
	transactionLock *sync.Mutex
}

func (s *Server) AddTransaction(tr blkparser.Tx) error {
	s.transactionLock.Lock()
	s.transactions = append(s.transactions, tr)
	s.transactionLock.Unlock()
	return nil
}
