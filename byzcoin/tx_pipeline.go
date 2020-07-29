package byzcoin

import (
	"bytes"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
	"sync"
)

// maxTxHashes of ClientTransactions are kept to early reject already sent
// ClientTransactions.
var maxTxHashes = 1000

// txPipeline gathers new ClientTransactions and VersionUpdate requests,
// and queues them up to be proposed as new blocks.
type txPipeline struct {
	ctxChan     chan ClientTransaction
	needUpgrade chan Version
	stopCollect chan bool
	newVersion  Version
	txQueue     []ClientTransaction
	wg          sync.WaitGroup
	processor   txProcessor
}

// newTxPipeline returns an initialized txPipeLine with a byzcoin-service
// enabled txProcessor.
func newTxPipeline(s *Service, latest *skipchain.SkipBlock) *txPipeline {
	return &txPipeline{
		ctxChan:     make(chan ClientTransaction, 200),
		needUpgrade: make(chan Version, 1),
		stopCollect: make(chan bool),
		wg:          sync.WaitGroup{},
		processor: &defaultTxProcessor{
			Service: s,
			scID:    latest.SkipChainID(),
			Mutex:   sync.Mutex{},
		},
	}
}

// start listens for new ClientTransactions and queues them up.
// It also listens for update requests and puts them in the queue.
// If a block is pending for approval,
// the next block is already in the channel.
// New ClientTransactions coming in during this time will still be appended
// to the block-proposition in the channel, until the proposition is too big.
func (p *txPipeline) start(currentState *proposedTransactions,
	stopSignal chan struct{}) {

	// Stores the known transaction-hashes to avoid double-inclusion of the
	// same transaction.
	var txHashes [][]byte

	// newBlock also serves as cache for the latest proposedTransactions: if the
	// new block hasn't been produced, it is legit to read the channel,
	// update the state, and write it back in.
	newBlock := make(chan *proposedTransactions, 1)
	blockSent := make(chan struct{}, 1)
	p.wg.Add(1)
	go p.createBlocks(newBlock, blockSent)

leaderLoop:
	for {
		select {
		case <-stopSignal:
			// Either a view-change or the node goes down.
			close(newBlock)
			break leaderLoop

		case <-blockSent:
			// A block has been proposed and either accepted or rejected.
			p.newVersion = 0

		case version := <-p.needUpgrade:
			// An upgrade of the system-version is needed.
			currVers, err := p.processor.GetVersion()
			if err != nil {
				log.Errorf("needUpgrade error: %v", err)
				continue
			}
			if version > currVers {
				p.newVersion = version
			}

		case tx := <-p.ctxChan:
			// A new ClientTransaction comes in - check if it's unique and
			// put it in the queue if it is.
			txh := tx.Instructions.HashWithSignatures()
			for _, txHash := range txHashes {
				if bytes.Equal(txHash, txh) {
					log.Lvl2("Got a duplicate transaction, ignoring it")
					continue leaderLoop
				}
			}
			txHashes = append(txHashes, txh)
			if len(txHashes) > maxTxHashes {
				txHashes = txHashes[len(txHashes)-maxTxHashes:]
			}

			p.txQueue = append(p.txQueue, tx)
		}

		// Check if a block is pending, fetch it if it's the case
		select {
		case nbState, ok := <-newBlock:
			if ok {
				currentState = nbState
			}
		default:
		}

		// For a version update, need to recover the ClientTransactions,
		// and send a version update
		if p.newVersion > 0 {
			newBlock <- &proposedTransactions{newVersion: p.newVersion,
				sst: currentState.sst}
			// This will be mostly a no-op in case there are no transactions
			// waiting...
			txs := make([]ClientTransaction, len(currentState.txs))
			for i, txRes := range currentState.txs {
				txs[i] = txRes.ClientTransaction
			}
			p.txQueue = append(txs, p.txQueue...)
			continue
		}

		// Add as many ClientTransactions as possible to the proposedTransactions
		// before the block gets too big, then put it in the channel.
		p.txQueue = currentState.addTransactions(p.processor, p.txQueue)
		if !currentState.isEmpty() {
			newBlock <- currentState.copy()
			currentState.reset()
		}
	}
	p.wg.Wait()
}

// createBlocks is the background routine that listens for new blocks and
// proposes them to the other nodes.
// Once a block is done, it signals it to the caller,
// so that eventual new blocks can be sent right away.
func (p *txPipeline) createBlocks(newBlock chan *proposedTransactions,
	blockSent chan struct{}) {
	defer p.wg.Done()
	for {
		inState, ok := <-newBlock
		if !ok {
			break
		}

		if inState.isVersionUpdate() {
			// Create an upgrade block for the next version
			err := p.processor.ProposeUpgradeBlock(inState.newVersion)
			if err != nil {
				// Only log the error as it won't prevent normal blocks
				// to be created.
				log.Error("failed to upgrade", err)
			}
		} else {
			// ProposeBlock sends the block to all other nodes.
			// It blocks until the new block has either been accepted or
			// rejected.
			err := p.processor.ProposeBlock(inState)
			if err != nil {
				log.Error("failed to propose block:", err)
			}
		}
		blockSent <- struct{}{}
	}
}

// txProcessor is the interface that must be implemented. It is used in the
// stateful pipeline txPipeline that takes transactions and creates blocks.
type txProcessor interface {
	// ProcessTx attempts to apply the given tx to the input state and then
	// produce new state(s). If the new tx is too big to fit inside a new
	// state, the function will return more states. Where the older states
	// (low index) must be committed before the newer state (high index)
	// can be used. The function should only return error when there is a
	// catastrophic failure, if the transaction is refused then it should
	// not return error, but mark the transaction's Accept flag as false.
	ProcessTx(*stagingStateTrie, ClientTransaction) (StateChanges,
		*stagingStateTrie, error)
	// ProposeBlock should take the input state and propose the block. The
	// function should only return when a decision has been made regarding
	// the proposal.
	ProposeBlock(*proposedTransactions) error
	// ProposeUpgradeBlock should create a barrier block between two Byzcoin
	// version so that future blocks will use the new version.
	ProposeUpgradeBlock(Version) error
	// GetBlockSize should return the maximum block size.
	GetBlockSize() int
	// Returns the current version of ByzCoin as per the stateTrie
	GetVersion() (Version, error)
}

// defaultTxProcessor is an implementation of txProcessor that uses a
// byzcoin-service to handle all requests.
// It is mainly a wrapper around the byzcoin calls.
type defaultTxProcessor struct {
	*Service
	scID skipchain.SkipBlockID
	sync.Mutex
}

func (s *defaultTxProcessor) ProcessTx(sst *stagingStateTrie,
	tx ClientTransaction) (StateChanges, *stagingStateTrie, error) {
	latest, err := s.db().GetLatestByID(s.scID)
	if err != nil {
		return nil, nil, xerrors.Errorf("couldn't get latest block: %v", err)
	}

	header, err := decodeBlockHeader(latest)
	if err != nil {
		return nil, nil, xerrors.Errorf("decoding header: %v", err)
	}

	tx = tx.Clone()
	tx.Instructions.SetVersion(header.Version)

	return s.processOneTx(sst, tx, s.scID, header.Timestamp)
}

func (s *defaultTxProcessor) ProposeBlock(state *proposedTransactions) error {
	config, err := state.sst.LoadConfig()
	if err != nil {
		return xerrors.Errorf("reading trie: %v", err)
	}
	_, err = s.createNewBlock(s.scID, &config.Roster, state.txs)
	return cothority.ErrorOrNil(err, "creating block")
}

func (s *defaultTxProcessor) ProposeUpgradeBlock(version Version) error {
	_, err := s.createUpgradeVersionBlock(s.scID, version)
	return cothority.ErrorOrNil(err, "creating block")
}

func (s *defaultTxProcessor) GetBlockSize() int {
	bcConfig, err := s.LoadConfig(s.scID)
	if err != nil {
		log.Error(s.ServerIdentity(), "couldn't get configuration - this is bad and probably "+
			"a problem with the database! ", err)
		return defaultMaxBlockSize
	}
	return bcConfig.MaxBlockSize
}

func (s *defaultTxProcessor) GetVersion() (Version, error) {
	st, err := s.Service.getStateTrie(s.scID)
	if err != nil {
		return -1, xerrors.Errorf("couldn't get version: %v", err)
	}
	return st.GetVersion(), nil
}

// proposedTransactions hold the proposal of the block to be sent out to the
// nodes.
// It can be updated with new transactions until is it sent to the nodes.
type proposedTransactions struct {
	sst        *stagingStateTrie
	scs        StateChanges
	txs        TxResults
	newVersion Version
}

// size returns the size of the transactions in this state,
// if they would be included in a block.
// This is not completely accurate, as it misses the data in the header.
// But we suppose that the header is small compared to the body.
func (s proposedTransactions) size(newTx TxResult) int {
	txs := append(s.txs, newTx)
	sb := skipchain.NewSkipBlock()
	body := &DataBody{TxResults: txs}
	var err error
	sb.Payload, err = protobuf.Encode(body)
	if err != nil {
		return 0
	}
	buf, err := protobuf.Encode(sb)
	if err != nil {
		return 0
	}
	return len(buf)
}

// reset removes all transactions and the resulting statechanges.
func (s *proposedTransactions) reset() {
	s.scs = []StateChange{}
	s.txs = []TxResult{}
	s.newVersion = 0
}

// copy creates a shallow copy the state, we don't have the need for deep copy
// yet.
func (s proposedTransactions) copy() *proposedTransactions {
	return &proposedTransactions{
		s.sst.Clone(),
		append([]StateChange{}, s.scs...),
		append([]TxResult{}, s.txs...),
		0,
	}
}

// isEmpty returns true if this proposedTransactions has neither a transaction
// nor a version update.
func (s proposedTransactions) isEmpty() bool {
	return len(s.txs) == 0 && s.newVersion == 0
}

// addTransactions runs the given ClientTransactions on the latest given
// proposedTransactions.
// Then it verifies if the resulting block would be too big,
// and if there is space, it adds the new ClientTransaction and the
// StateChanges to the proposedTransactions.
func (s *proposedTransactions) addTransactions(p txProcessor,
	txs []ClientTransaction) []ClientTransaction {

	for len(txs) > 0 {
		tx := txs[0]
		newScs, newSst, err := p.ProcessTx(s.sst, tx)
		txRes := TxResult{
			ClientTransaction: tx,
			Accepted:          err == nil,
		}

		// If the resulting block would be too big,
		// simply skip this and all remaining transactions.
		if s.size(txRes) > p.GetBlockSize() {
			break
		}

		if txRes.Accepted {
			s.sst = newSst
			s.scs = append(s.scs, newScs...)
		}
		s.txs = append(s.txs, txRes)
		txs = txs[1:]
	}
	return txs
}

// isVersionUpdate returns whether this proposedTransactions requests a new version.
func (s proposedTransactions) isVersionUpdate() bool {
	return s.newVersion > 0
}
