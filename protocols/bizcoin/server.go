package bizcoin

// Server is the longterm control service that listens for transactions and
// dispatch them to a new BizCoin for each new signing that we want to do.
// It creates the BizCoin protocols and run them. only used by the root since
// only the root pariticipates to the creation of the block.
type Server struct {
	// transaction pool where all the incoming transactions are stored
	transaction_pool []blkparser.Tx
	// lock associated
	transactionLock *sync.Mutex
}

func (s *Server) AddTransaction(tr blkparser.Tx) {
	bz.transactionLock.Lock()
	bz.transactions = append(bz.transaction_pool, tr)
	bz.transactionLock.Unlock()
}
