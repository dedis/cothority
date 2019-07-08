package byzcoin

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// txProcessor is the interface that must be implemented. It is used in the
// stateful pipeline txPipeline that takes transactions and creates blocks.
type txProcessor interface {
	// CollectTx implements a blocking function that returns transactions
	// that should go into new blocks. These transactions are not verified.
	CollectTx() ([]ClientTransaction, error)
	// ProcessTx attempts to apply the given tx to the input state and then
	// produce new state(s). If the new tx is too big to fit inside a new
	// state, the function will return more states. Where the older states
	// (low index) must be committed before the newer state (high index)
	// can be used. The function should only return error when there is a
	// catastrophic failure, if the transaction is refused then it should
	// not return error, but mark the transaction's Accept flag as false.
	ProcessTx(ClientTransaction, *txProcessorState) ([]*txProcessorState, error)
	// ProposeBlock should take the input state and propose the block. The
	// function should only return when a decision has been made regarding
	// the proposal.
	ProposeBlock(*txProcessorState) error
	// GetLatestGoodState should return the latest state that the processor
	// trusts.
	GetLatestGoodState() *txProcessorState
	// GetBlockSize should return the maximum block size.
	GetBlockSize() int
	// GetInterval should return the block interval.
	GetInterval() time.Duration
	// Stop stops the txProcessor. Once it is called, the caller should not
	// expect the other functions in the interface to work as expected.
	Stop()
}

type txProcessorState struct {
	sst *stagingStateTrie

	// Below are changes that were made that led up to the state in sst
	// from the starting point.
	scs     StateChanges
	txs     TxResults
	txsSize int
}

func (s *txProcessorState) size() int {
	if s.txsSize == 0 {
		body := &DataBody{TxResults: s.txs}
		payload, err := protobuf.Encode(body)
		if err != nil {
			return 0
		}
		s.txsSize = len(payload)
	}
	return s.txsSize
}

func (s *txProcessorState) reset() {
	s.scs = []StateChange{}
	s.txs = []TxResult{}
	s.txsSize = 0
}

// copy creates a shallow copy the state, we don't have the need for deep copy
// yet.
func (s *txProcessorState) copy() *txProcessorState {
	return &txProcessorState{
		s.sst.Clone(),
		append([]StateChange{}, s.scs...),
		append([]TxResult{}, s.txs...),
		s.txsSize,
	}
}

type defaultTxProcessor struct {
	stopCollect chan bool
	scID        skipchain.SkipBlockID
	*Service
}

func (s *defaultTxProcessor) CollectTx() ([]ClientTransaction, error) {
	// Need to update the config, as in the meantime a new block should have
	// arrived with a possible new configuration.
	bcConfig, err := s.LoadConfig(s.scID)
	if err != nil {
		log.Error(s.ServerIdentity(), "couldn't get configuration - this is bad and probably "+
			"a problem with the database! "+err.Error())
		return nil, err
	}

	_, isNotProcessingBlock := s.skService().WaitBlock(s.scID, nil)

	latest, err := s.db().GetLatestByID(s.scID)
	if err != nil {
		log.Errorf("Error while searching for %x", s.scID[:])
		log.Error("DB is in bad state and cannot find skipchain anymore: " + err.Error() +
			" This function should never be called on a skipchain that does not exist.")
		return nil, err
	}

	log.Lvlf3("%s: Starting new block %d for chain %x", s.ServerIdentity(), latest.Index+1, s.scID)
	tree := bcConfig.Roster.GenerateNaryTree(len(bcConfig.Roster.List))

	proto, err := s.CreateProtocol(collectTxProtocol, tree)
	if err != nil {
		log.Error(s.ServerIdentity(), "Protocol creation failed with error: "+err.Error()+
			" This panic indicates that there is most likely a programmer error,"+
			" e.g., the protocol does not exist."+
			" Hence, we cannot recover from this failure without putting"+
			" the server in a strange state, so we panic.")
		return nil, err
	}
	root := proto.(*CollectTxProtocol)
	root.SkipchainID = s.scID
	root.LatestID = latest.Hash
	// When a block is processed, we prevent conodes to send us back transactions
	// until the next collection.
	if !isNotProcessingBlock {
		root.MaxNumTxs = 0
	}

	if err := root.Start(); err != nil {
		log.Error(s.ServerIdentity(), "Failed to start the protocol with error: "+err.Error()+
			" Start() only returns an error when the protocol is not initialised correctly,"+
			" e.g., not all the required fields are set."+
			" If you see this message then there may be a programmer error.")
		return nil, err
	}

	// When we poll, the child nodes must reply within half of the block
	// interval, because we'll use the other half to process the
	// transactions.
	protocolTimeout := time.After(bcConfig.BlockInterval / 2)

	var txs []ClientTransaction
collectTxLoop:
	for {
		select {
		case newTxs, more := <-root.TxsChan:
			if more {
				for _, ct := range newTxs {
					txsz := txSize(TxResult{ClientTransaction: ct})
					if txsz < bcConfig.MaxBlockSize {
						txs = append(txs, ct)
					} else {
						log.Lvl2(s.ServerIdentity(), "dropping collected transaction with length", txsz)
					}
				}
			} else {
				break collectTxLoop
			}
		case <-protocolTimeout:
			log.Lvl2(s.ServerIdentity(), "timeout while collecting transactions from other nodes")
			close(root.Finish)
			break collectTxLoop
		case <-s.stopCollect:
			log.Lvl2(s.ServerIdentity(), "abort collection of transactions")
			close(root.Finish)
			return txs, nil
		}
	}

	return txs, nil
}

func (s *defaultTxProcessor) ProcessTx(tx ClientTransaction, inState *txProcessorState) ([]*txProcessorState, error) {
	scsOut, sstOut, err := s.processOneTx(inState.sst, tx)

	// try to create a new state
	newState := func() *txProcessorState {
		if err != nil {
			return &txProcessorState{
				inState.sst,
				inState.scs,
				append(inState.txs, TxResult{tx, false}),
				0,
			}
		}
		return &txProcessorState{
			sstOut,
			append(inState.scs, scsOut...),
			append(inState.txs, TxResult{tx, true}),
			0,
		}
	}()

	// we're within the block size, so return one state
	if s.GetBlockSize() > newState.size() {
		return []*txProcessorState{newState}, nil
	}

	// if the new state is too big, we split it
	newStates := []*txProcessorState{inState.copy()}
	if err != nil {
		newStates = append(newStates, &txProcessorState{
			inState.sst,
			inState.scs,
			[]TxResult{{tx, false}},
			0,
		})
	} else {
		newStates = append(newStates, &txProcessorState{
			sstOut,
			scsOut,
			[]TxResult{{tx, true}},
			0,
		})
	}
	return newStates, nil
}

// ProposeBlock basically calls s.createNewBlock which might block. There is
// nothing we can do about it other than waiting for the timeout.
func (s *defaultTxProcessor) ProposeBlock(state *txProcessorState) error {
	config, err := LoadConfigFromTrie(state.sst)
	if err != nil {
		return err
	}
	_, err = s.createNewBlock(s.scID, &config.Roster, state.txs)
	return err
}

func (s *defaultTxProcessor) GetInterval() time.Duration {
	bcConfig, err := s.LoadConfig(s.scID)
	if err != nil {
		log.Error(s.ServerIdentity(), "couldn't get configuration - this is bad and probably "+
			"a problem with the database! "+err.Error())
		return defaultInterval
	}
	return bcConfig.BlockInterval
}

func (s *defaultTxProcessor) GetLatestGoodState() *txProcessorState {
	st, err := s.getStateTrie(s.scID)
	if err != nil {
		// A good state must exist because we're working on a known
		// skipchain. If there is an error, then the database must've
		// failed, so there is nothing we can do to recover so we
		// panic.
		panic(fmt.Sprintf("failed to get a good state: %v", err))
	}
	return &txProcessorState{
		sst: st.MakeStagingStateTrie(),
	}
}

func (s *defaultTxProcessor) GetBlockSize() int {
	bcConfig, err := s.LoadConfig(s.scID)
	if err != nil {
		log.Error(s.ServerIdentity(), "couldn't get configuration - this is bad and probably "+
			"a problem with the database! "+err.Error())
		return defaultMaxBlockSize
	}
	return bcConfig.MaxBlockSize
}

func (s *defaultTxProcessor) Stop() {
	close(s.stopCollect)
}

type txPipeline struct {
	wg        sync.WaitGroup
	processor txProcessor
}

func (p *txPipeline) start(initialState *txProcessorState, stopSignal chan bool) {
	stopCollect := make(chan bool)
	ctxChan := p.collectTx(stopCollect)
	p.processTxs(ctxChan, initialState)

	<-stopSignal
	close(stopCollect)
	p.processor.Stop()
	p.wg.Wait()
}

func (p *txPipeline) collectTx(stopChan chan bool) <-chan ClientTransaction {
	outChan := make(chan ClientTransaction, 200)
	// set the polling interval to half of the block interval
	go func() {
		p.wg.Add(1)
		defer p.wg.Done()
		for {
			interval := p.processor.GetInterval()
			select {
			case <-stopChan:
				log.Lvl3("stopping tx collector")
				close(outChan)
				return
			case <-time.After(interval / 2):
				txs, err := p.processor.CollectTx()
				if err != nil {
					log.Error("failed to collect transactions")
				}
				for _, tx := range txs {
					select {
					case outChan <- tx:
						// channel not full, do nothing
					default:
						log.Warn("dropping transactions because there are too many")
					}
				}
			}
		}
	}()
	return outChan
}

var maxTxHashes = 1000

// processTxs consumes transactions and computes the new txResults
func (p *txPipeline) processTxs(txChan <-chan ClientTransaction, initialState *txProcessorState) {
	var proposing bool
	// always use the latest one when adding new
	currentState := []*txProcessorState{initialState}
	proposalResult := make(chan error, 1)
	getInterval := func() <-chan time.Time {
		interval := p.processor.GetInterval()
		return time.After(interval)
	}
	go func() {
		p.wg.Add(1)
		defer p.wg.Done()
		intervalChan := getInterval()
		var txHashes [][]byte
	leaderLoop:
		for {
			select {
			case tx, ok := <-txChan:
				if !ok {
					log.Lvl3("stopping txs processor")
					return
				}
				txh := tx.Instructions.Hash()
				for _, txHash := range txHashes {
					if bytes.Compare(txHash, txh) == 0 {
						log.Lvl2("Got a duplicate transaction, ignoring it")
						continue leaderLoop
					}
				}
				txHashes = append(txHashes, txh)
				if len(txHashes) > maxTxHashes {
					txHashes = txHashes[len(txHashes)-maxTxHashes:]
				}

				// when processing, we take the latest state
				// (the last one) and then apply the new transaction to it
				newStates, err := p.processor.ProcessTx(tx, currentState[len(currentState)-1])
				if err != nil {
					log.Error("processing transaction failed with error: " + err.Error())
				} else {
					// remove the last one from currentState because
					// it might be getting updated and the append newStates
					currentState = append(currentState[:len(currentState)-1], newStates...)
				}
			case <-intervalChan:
				// update the interval every time because it might've changed
				intervalChan = getInterval()

				// wait for the next interval if there are no changes
				// we do not check for the length because currentState
				// should always be non-empty, otherwise it's a
				// programmer error
				if len(currentState[0].txs) == 0 {
					break
				}

				// if we're already proposing a block, then don't propose another one yet,
				// because the followers won't be able to verify
				if proposing {
					log.Warn("block proposal is taking a longer time than the interval to complete," +
						" consider using a longer block interval or check your network connection")
					break
				}
				proposing = true

				// find the right state and propose it in the block
				var inState *txProcessorState
				currentState, inState = proposeInputState(currentState)

				go func(state *txProcessorState) {
					p.wg.Add(1)
					defer p.wg.Done()
					if state == nil {
						proposalResult <- nil
					} else {
						// NOTE: ProposeBlock might block for a long time,
						// but there's nothing we can do about it at the moment
						// other than waiting for the timeout.
						if err := p.processor.ProposeBlock(state); err != nil {
							log.Error("failed to propose block:", err)
							proposalResult <- err
						} else {
							proposalResult <- nil
						}
					}
				}(inState)
			case err := <-proposalResult:
				// only the ProposeBlock sends back results and it sends only one
				proposing = false
				if err != nil {
					log.Error("reverting to last known state because proposal refused:", err)
					currentState = []*txProcessorState{p.processor.GetLatestGoodState()}
				}
			}
		}
	}()
}

// proposeInputState generates the next input state that is used in
// ProposeBlock. It returns a new state for the pipeline and the state for
// ProposeBlock.
func proposeInputState(currStates []*txProcessorState) ([]*txProcessorState, *txProcessorState) {
	// currStates should always be non empty
	// I wish we had a type like Data.List.NonEmpty
	if len(currStates) == 1 {
		inState := currStates[0].copy()
		currStates[0].reset()
		return currStates, inState
	}
	inState := currStates[0]
	return currStates[1:], inState
}
