package byzcoin

import (
	"sync"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain/blkparser"
)

// BlockServer is a struct where Client can connect and that instantiate ByzCoin
// protocols when needed.
type BlockServer interface {
	AddTransaction(blkparser.Tx)
	Instantiate(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error)
}

// Server is the long-term control service that listens for transactions and
// dispatch them to a new ByzCoin for each new signing that we want to do.
// It creates the ByzCoin protocols and run them. only used by the root since
// only the root participates to the creation of the block.
type Server struct {
	// transactions pool where all the incoming transactions are stored
	transactions []blkparser.Tx
	// lock associated
	transactionLock sync.Mutex
	// how many transactions should we give to an instance
	blockSize int
	timeOutMs uint64
	fail      uint
	// blockSignatureChan is the channel used to pass out the signatures that
	// ByzCoin's instances have made
	blockSignatureChan chan BlockSignature
	// enoughBlock signals the server we have enough
	// no comments..
	transactionChan chan blkparser.Tx
	requestChan     chan bool
	responseChan    chan []blkparser.Tx
}

// NewByzCoinServer returns a new fresh ByzCoinServer. It must be given the blockSize in order
// to efficiently give the transactions to the ByzCoin instances.
func NewByzCoinServer(blockSize int, timeOutMs uint64, fail uint) *Server {
	s := &Server{
		blockSize:          blockSize,
		timeOutMs:          timeOutMs,
		fail:               fail,
		blockSignatureChan: make(chan BlockSignature),
		transactionChan:    make(chan blkparser.Tx),
		requestChan:        make(chan bool),
		responseChan:       make(chan []blkparser.Tx),
	}
	go s.listenEnoughBlocks()
	return s
}

// AddTransaction add a new transactions to the list of transactions to commit
func (s *Server) AddTransaction(tr blkparser.Tx) {
	s.transactionChan <- tr
}

// ListenClientTransactions will bind to a port a listen for incoming connection
// from clients. These client will be able to pass the transactions to the
// server.
func (s *Server) ListenClientTransactions() {
	panic("not implemented yet")
}

// Instantiate takes blockSize transactions and create the byzcoin instances.
func (s *Server) Instantiate(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	// wait until we have enough blocks
	currTransactions := s.WaitEnoughBlocks()
	dbg.Lvl2("Instantiate ByzCoin Round with", len(currTransactions), "transactions")
	pi, err := NewByzCoinRootProtocol(node, currTransactions, s.timeOutMs, s.fail)

	return pi, err
}

// BlockSignaturesChan returns a channel that is given each new block signature as
// soon as they are arrive (Wether correct or not).
func (s *Server) BlockSignaturesChan() <-chan BlockSignature {
	return s.blockSignatureChan
}

func (s *Server) onDoneSign(blk BlockSignature) {
	s.blockSignatureChan <- blk
}

// WaitEnoughBlocks is called to wait on the server until it has enough
// transactions to make a block
func (s *Server) WaitEnoughBlocks() []blkparser.Tx {
	s.requestChan <- true
	transactions := <-s.responseChan
	return transactions
}

func (s *Server) listenEnoughBlocks() {
	// TODO the server should have a transaction pool instead:
	var transactions []blkparser.Tx
	var want bool
	for {
		select {
		case tr := <-s.transactionChan:
			// FIXME this will lead to a very large slice if the client sends many
			if len(transactions) < s.blockSize {
				transactions = append(transactions, tr)
			}
			if want {
				if len(transactions) >= s.blockSize {
					s.responseChan <- transactions[:s.blockSize]
					transactions = transactions[s.blockSize:]
					want = false
				}
			}
		case <-s.requestChan:
			want = true
			if len(transactions) >= s.blockSize {
				s.responseChan <- transactions[:s.blockSize]
				transactions = transactions[s.blockSize:]
				want = false
			}
		}
	}
}
