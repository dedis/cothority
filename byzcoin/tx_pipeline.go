package byzcoin

import (
	"fmt"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"time"
)

// txProcessor TODO ...
type txProcessor interface {
	CollectTx() ([]ClientTransaction, error)
	// ProcessTx should only return error when there is a catastrophic failure,
	// if the transaction is refused then it should not return error, but mark
	// the transaction's Accept flag as false.
	ProcessTx(ClientTransaction, *txProcessorState) ([]*txProcessorState, error)
	// ProposeBlock should be the only stateful operation because if it returns
	// successfully then it will store the new block.
	ProposeBlock(*txProcessorState) error
	GetInterval() (time.Duration, error)
	GetLatestGoodState() (*txProcessorState, error)
	GetBlockSize() int
	Stop()
}

type txProcessorState struct {
	// TODO we should say where the starting point is
	sst *stagingStateTrie

	// metadata, changes that were made that led up to the state in sst from some starting point
	scs   StateChanges
	txs   TxResults
}

func (s *txProcessorState) size() int {
	// TODO is there a better way to estimate the size before creating the block?
	body := &DataBody{TxResults: s.txs}
	payload, err := protobuf.Encode(body)
	if err != nil {
		return 0
	}
	return len(payload)
}

func (s *txProcessorState) reset() {
	s.scs = []StateChange{}
	s.txs = []TxResult{}
}

func (s *txProcessorState) copy() *txProcessorState {
	// TODO this is not a deep copy because StateChanges and TxResults contain reference types
	return &txProcessorState{
		s.sst.Clone(),
		append([]StateChange{}, s.scs...),
		append([]TxResult{}, s.txs...),
	}
}

type defaultTxProcessor struct {
	stopCollect chan bool
	stopProcess chan bool
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
	if err := root.Start(); err != nil {
		log.Error(s.ServerIdentity(), "Failed to start the protocol with error: "+err.Error()+
			" Start() only returns an error when the protocol is not initialised correctly,"+
			" e.g., not all the required fields are set."+
			" If you see this message then there may be a programmer error.")
		return nil, err
	}

	// When we poll, the child nodes must reply within half of the block interval,
	// because we'll use the other half to process the transactions.
	// TODO collection protocol may not depend on block interval anymore
	protocolTimeout := time.After(bcConfig.BlockInterval / 2)

	var txs []ClientTransaction
collectTxLoop:
	for {
		select {
		case newTxs, more := <-root.TxsChan:
			if more {
				for _, ct := range newTxs {
					// TODO ?????
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

// ProcessTx attempts to apply the given tx to inState and then produce a new state.
// If the new tx is too big to fit inside a new state, the function will return more states.
// Where the older states (low index) must be committed before the newer state (high index) can be used.
func (s *defaultTxProcessor) ProcessTx(tx ClientTransaction, inState *txProcessorState) ([]*txProcessorState, error) {
	scsOut, sstOut, err := s.processOneTx(inState.sst, tx)

	// try to create a new state
	newState := func() *txProcessorState {
		if err != nil {
			return &txProcessorState{
				inState.sst,
				inState.scs,
				append(inState.txs, TxResult{tx, false}),
			}
		}
		return &txProcessorState{
			sstOut,
			append(inState.scs, scsOut...),
			append(inState.txs, TxResult{tx, true}),
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
		})
	} else {
		newStates = append(newStates, &txProcessorState{
			sstOut,
			scsOut,
			[]TxResult{{tx, true}},
		})
	}
	return newStates, nil
}

// ProposeBlock basically calls s.createNewBlock which might block. There is nothing we can do about it other than
// waiting for the timeout.
func (s *defaultTxProcessor) ProposeBlock(state *txProcessorState) error {
	config, err := LoadConfigFromTrie(state.sst)
	if err != nil {
		return err
	}
	_, err = s.createNewBlock(s.scID, &config.Roster, state.txs)
	return err
}

func (s *defaultTxProcessor) GetInterval() (time.Duration, error) {
	bcConfig, err := s.LoadConfig(s.scID)
	if err != nil {
		log.Error(s.ServerIdentity(), "couldn't get configuration - this is bad and probably "+
			"a problem with the database! "+err.Error())
		return 0, err
	}
	return bcConfig.BlockInterval, nil
}

func (s *defaultTxProcessor) GetLatestGoodState() (*txProcessorState, error) {
	st, err := s.getStateTrie(s.scID)
	if err != nil {
		return nil, err
	}
	return &txProcessorState{
		sst: st.MakeStagingStateTrie(),
	}, nil
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
	close(s.stopProcess)
}

type txPipeline struct {
	processor txProcessor
}

func (p *txPipeline) start(initialState *txProcessorState) chan bool {
	stopChan := make(chan bool)

	ctxChan, stopChan1 := p.collectTx()
	stopChan2 := p.processTxs(ctxChan, initialState)

	go func() {
		<-stopChan
		close(stopChan1)
		close(stopChan2)
	}()

	return stopChan
}

func (p *txPipeline) collectTx() (<-chan ClientTransaction, chan<- bool) {
	stopChan := make(chan bool, 1)
	outChan := make(chan ClientTransaction, 200)
	// set the polling interval to half of the block interval
	go func() {
		for {
			interval, err := p.processor.GetInterval()
			if err != nil {
				log.Error("failed to get interval: " + err.Error())
				interval = time.Second
			}
			select {
			case <-stopChan:
				log.Lvl3("stopping tx collector")
				return
			case <-time.After(interval / 2):
				txs, err := p.processor.CollectTx()
				if err != nil {
					log.Error("failed to collect transactions")
				}
				for _, tx := range txs {
					outChan <- tx
				}
			}
		}
	}()
	return outChan, stopChan
}

// processTxs consumes transactions and computes the
func (p *txPipeline) processTxs(txChan <-chan ClientTransaction, initialState *txProcessorState) chan<- bool {
	stopChan := make(chan bool, 1)
	// always use the latest one when adding new
	currentState := []*txProcessorState{initialState}
	proposalResult := make(chan error, 1)
	getInterval := func() <-chan time.Time {
		interval, err := p.processor.GetInterval()
		if err != nil {
			log.Error("failed to get interval: " + err.Error())
			interval = time.Second
		}
		return time.After(interval)
	}
	go func() {
		intervalChan := getInterval()
		for {
			select {
			case <-stopChan:
				log.Lvl3("stopping txs processor")
				return
			case tx := <-txChan:
				// when processing, we take the latest state (the last one) and then apply the new transaction to it
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
				// TODO check length
				if len(currentState[0].txs) == 0 {
					break
				}

				var inState *txProcessorState
				currentState, inState = proposeInputState(currentState)

				go func(state *txProcessorState) {
					if state == nil {
						proposalResult <- nil
					} else {
						// ProposeBlock might block, but there's nothing we can do about it at the moment
						// other than waiting for the timeout.
						if err := p.processor.ProposeBlock(state); err != nil {
							log.Error("failed to propose block: " + err.Error())
							proposalResult <- err
						} else {
							proposalResult <- nil
						}
					}
				}(inState)
			case err := <-proposalResult:
				if err != nil {
					log.Error("reverting to last known state because proposal refused:", err.Error())
					newCurrentState, err := p.processor.GetLatestGoodState()
					if err != nil || currentState == nil {
						// A good state must exist because we're working on a known skipchain, it must at least
						// contain the genesis state. If there is an error, then the database must've failed,
						// so there is nothing we can do to recover so we panic.
						panic(fmt.Sprintf("failed to get a good state: %v", err))
					}
					currentState = []*txProcessorState{newCurrentState}
				}
			}
		}
	}()
	return stopChan
}

// proposeInputState generates the next input state that is used in ProposeBlock.
// It returns a new state for the pipeline and the state for ProposeBlock.
func proposeInputState(currStates []*txProcessorState) ([]*txProcessorState, *txProcessorState) {
	if len(currStates) == 0 {
		panic("wut")
	}
	if len(currStates) == 1 {
		inState := currStates[0].copy()
		currStates[0].reset()
		return currStates, inState
	}
	inState := currStates[0]
	return currStates[1:], inState
}
