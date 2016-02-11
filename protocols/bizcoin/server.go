package bizcoin

import (
	"sync"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/bizcoin/blockchain/blkparser"
	"github.com/satori/go.uuid"
)

// Control is just a sample of how a Control / Server / (what would be the right
// name !!??) would look like if we were to go forward with the idea in the
// issue https://github.com/dedis/cothority/issues/211
type Control interface {
	Instantiate(*sda.Node) (sda.ProtocolInstance, error)
}

// something like this should happens just after.i.e. the Setup() part
// sda.RegisterControl("BizCoin",NewServer)
//
// Via the CLI, we could issue ```sda start BizCoin```and it would look if there
// is a reference for this Control at the name and initialize it.

// Here is the real (non-fictional) stuff ;)

// Server is the longterm control service that listens for transactions and
// dispatch them to a new BizCoin for each new signing that we want to do.
// It creates the BizCoin protocols and run them. only used by the root since
// only the root pariticipates to the creation of the block.
type Server struct {
	// transactions pool where all the incoming transactions are stored
	transactions []blkparser.Tx
	// lock associated
	transactionLock *sync.Mutex
	// how many transactions should we give to an instance
	blockSize int
	// all the protocols bizcoin he generated.Map from RoundID <-> BizCoin
	// protocol instance.
	instances map[uuid.UUID]*BizCoin
	// blockSignatureChan is the channel used to pass out the signatures that
	// BizCoin's instances have made
	blockSignatureChan chan BlockSignature
	// communicate an incoming transaction by this channel
	transactionChan chan blkparser.Tx
	// communicate if there is a go-routine waiting for enough transactions
	requestChan chan bool
	// communicate (that we have enough) transactions
	responseChan chan []blkparser.Tx
}

// NewServer returns a new fresh Server. It must be given the blockSize in order
// to efficiently give the transactions to the BizCoin instances.
func NewServer(blockSize int) *Server {
	s := &Server{
		transactionLock:    new(sync.Mutex),
		blockSize:          blockSize,
		instances:          make(map[uuid.UUID]*BizCoin),
		blockSignatureChan: make(chan BlockSignature),
		transactionChan:    make(chan blkparser.Tx),
		requestChan:        make(chan bool),
		responseChan:       make(chan []blkparser.Tx),
	}
	go s.listenEnoughBlocks()
	return s
}

// AddTransaction can be used to simulate the client(s) sending a single
// transaction (without a network connection)
func (s *Server) AddTransaction(tr blkparser.Tx) error {
	s.transactionChan <- tr
	return nil
}

// ListenClientTransactions will bind to a port a listen for incoming connection
// from clients. These client will be able to pass the transactions to the
// server.
func (s *Server) ListenClientTransactions() {
	panic("not implemented yet")
}

// Instantiate takes blockSize transactions and create the bizcoin instances.
func (s *Server) Instantiate(node *sda.Node) (sda.ProtocolInstance, error) {
	// wait until we have enough blocks
	currTransactions := s.waitEnoughBlocks()
	dbg.Lvl3("Instantiate BizCoin Round with", len(currTransactions), " transactions")
	pi, err := NewBizCoinRootProtocol(node, currTransactions)
	node.SetProtocolInstance(pi)
	pi.RegisterOnDone(s.onDone)

	go pi.Start()

	return pi, err
}

// BlockSignaturesChan returns a channel that can be used to be notified when a
// signature on a blok is ready
// Used in simulation.go
func (s *Server) BlockSignaturesChan() <-chan BlockSignature {
	return s.blockSignatureChan
}

func (s *Server) onDone(blk BlockSignature) {
	s.blockSignatureChan <- blk
}

func (s *Server) waitEnoughBlocks() []blkparser.Tx {
	dbg.Lvl4("Releasing requestChan")
	s.requestChan <- true
	dbg.Lvl4("After requestChan released")
	transactions := <-s.responseChan
	dbg.Lvl4("After responseChan consumed")
	return transactions
}

func (s *Server) listenEnoughBlocks() {
	var transactions []blkparser.Tx
	var want bool
	for {
		select {
		case tr := <-s.transactionChan:
			transactions = append(transactions, tr)
			if want {
				dbg.Lvl4("Added new transaction, if we have enough we respond with them")
				if len(transactions) >= s.blockSize {
					dbg.Lvl4("will respond with enough transactions")
					s.responseChan <- transactions[:s.blockSize]
					transactions = transactions[s.blockSize:]
					want = false
				}
			}
		case <-s.requestChan:
			dbg.Lvl4("Requested more transactions")
			want = true
			if len(transactions) >= s.blockSize {
				dbg.Lvl4("will respond with enough transactions")
				s.responseChan <- transactions[:s.blockSize]
				transactions = transactions[s.blockSize:]
				want = false
			}
		}
	}
}
