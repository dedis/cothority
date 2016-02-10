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
	// enoughBlock signals the server we have enough
	// no comments..
	enoughBlock    chan bool
	notEnoughBlock chan bool
	requestChan    chan bool
}

// NewServer returns a new fresh Server. It must be given the blockSize in order
// to efficiently give the transactions to the BizCoin instances.
func NewServer(blockSize int) *Server {
	s := &Server{
		transactionLock:    new(sync.Mutex),
		blockSize:          blockSize,
		instances:          make(map[uuid.UUID]*BizCoin),
		blockSignatureChan: make(chan BlockSignature),
		enoughBlock:        make(chan bool),
		notEnoughBlock:     make(chan bool),
		requestChan:        make(chan bool),
	}
	go s.listenEnoughBlocks()
	return s
}

func (s *Server) AddTransaction(tr blkparser.Tx) error {
	s.transactionLock.Lock()
	s.transactions = append(s.transactions, tr)
	dbg.Print("Are there tranactions?")
	if len(s.transactions) >= s.blockSize {
		//	dbg.LLvl2("Enough is enough ... ................. ")
		s.enoughBlock <- true
	} else {
		s.notEnoughBlock <- true
	}
	s.transactionLock.Unlock()
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
	s.waitEnoughBlocks()
	dbg.LLvl2("Enough blocks... ................ we are starting")
	var currTransactions []blkparser.Tx
	s.transactionLock.Lock()
	defer s.transactionLock.Unlock()
	currTransactions = s.transactions[:s.blockSize]
	s.transactions = s.transactions[s.blockSize:]
	dbg.Lvl1("Instantiate BizCoin Round with", len(currTransactions), " transactions")
	pi, err := NewBizCoinRootProtocol(node, currTransactions)
	node.SetProtocolInstance(pi)
	pi.RegisterOnDone(s.onDone)

	go pi.Start()

	return pi, err
}

// BlockSignature returns a channel that is given each new block signature as
// soon as they are arrive (Wether correct or not).
func (s *Server) BlockSignaturesChan() <-chan BlockSignature {
	return s.blockSignatureChan
}

func (s *Server) onDone(blk BlockSignature) {
	s.blockSignatureChan <- blk
}

func (s *Server) waitEnoughBlocks() {
	dbg.Print("Consuming requestChan ............")
	<-s.requestChan
}
func (s *Server) listenEnoughBlocks() {
	var enoughRoutine bool
	for {
		select {
		case <-s.enoughBlock:
			if !enoughRoutine {
				dbg.Print("enoughBlock----------")
				go s.signalEnough()
				enoughRoutine = true
			}
		case <-s.notEnoughBlock:
			dbg.Print("s.notEnoughBlock---------------")
			enoughRoutine = false
		}
	}
}

func (s *Server) signalEnough() {
	dbg.Print("signalEnough: writing to chan -----------------")
	s.requestChan <- true
}

/*func (s *Server) waitEnoughBlocks() {*/
//s.requestLock.Lock()
//s.requestBlocks = true
//s.requestLock.Unlock()
//<-s.requestChan
//s.requestLock.Lock()
//s.requestBlocks = false
//s.requestLock.Unlock()

//}

//func (s *Server) listenEnoughBlocks() {
//for {
//select {
//case <-s.enoughBlock:
//s.requestLock.Lock()
//if s.requestBlocks {
//s.requestChan <- true
//}
//s.requestLock.Unlock()
//}
//}
/*}*/
