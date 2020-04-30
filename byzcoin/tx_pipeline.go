package byzcoin

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// collectTxResult contains the aggregated response of the conodes to the
// collectTx protocol.
type collectTxResult struct {
	Txs           []ClientTransaction
	CommonVersion Version
}

// txProcessor is the interface that must be implemented. It is used in the
// stateful pipeline txPipeline that takes transactions and creates blocks.
type txProcessor interface {
	// CollectTx implements a blocking function that returns transactions
	// that should go into new blocks. These transactions are not verified.
	CollectTx() (*collectTxResult, error)
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
	// ProposeUpgradeBlock should create a barrier block between two Byzcoin
	// version so that future blocks will use the new version.
	ProposeUpgradeBlock(Version) error
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
	*Service
	stopCollect chan bool
	scID        skipchain.SkipBlockID
	latest      *skipchain.SkipBlock
	sync.Mutex
}

func (s *defaultTxProcessor) CollectTx() (*collectTxResult, error) {
	// Need to update the config, as in the meantime a new block should have
	// arrived with a possible new configuration.
	bcConfig, err := s.LoadConfig(s.scID)
	if err != nil {
		log.Error(s.ServerIdentity(), "couldn't get configuration - this is bad and probably "+
			"a problem with the database! ", err)
		return nil, xerrors.Errorf("reading config: %v", err)
	}

	if s.skService().ChainIsProcessing(s.scID) {
		// When a block is processed,
		// return immediately without processing any tx from the nodes.
		return &collectTxResult{Txs: nil, CommonVersion: 0}, nil
	}

	latest, err := s.db().GetLatestByID(s.scID)
	if err != nil {
		log.Errorf("Error while searching for %x", s.scID[:])
		log.Error("DB is in bad state and cannot find skipchain anymore."+
			" This function should never be called on a skipchain that does not exist.", err)
		return nil, xerrors.Errorf("reading latest: %v", err)
	}

	// Keep track of the latest block for the processing
	s.Lock()
	s.latest = latest
	s.Unlock()

	log.Lvlf3("%s: Starting new block %d (%x) for chain %x", s.ServerIdentity(), latest.Index+1, latest.Hash, s.scID)
	tree := bcConfig.Roster.GenerateNaryTree(len(bcConfig.Roster.List))

	proto, err := s.CreateProtocol(collectTxProtocol, tree)
	if err != nil {
		log.Error(s.ServerIdentity(), "Protocol creation failed with error."+
			" This panic indicates that there is most likely a programmer error,"+
			" e.g., the protocol does not exist."+
			" Hence, we cannot recover from this failure without putting"+
			" the server in a strange state, so we panic.", err)
		return nil, xerrors.Errorf("creating protocol: %v", err)
	}
	root := proto.(*CollectTxProtocol)
	root.SkipchainID = s.scID
	root.LatestID = latest.Hash

	log.Lvl3("Asking", root.Roster().List, "for Txs")
	if err := root.Start(); err != nil {
		log.Error(s.ServerIdentity(), "Failed to start the protocol with error."+
			" Start() only returns an error when the protocol is not initialised correctly,"+
			" e.g., not all the required fields are set."+
			" If you see this message then there may be a programmer error.", err)
		return nil, xerrors.Errorf("starting protocol: %v", err)
	}

	// When we poll, the child nodes must reply within half of the block
	// interval, because we'll use the other half to process the
	// transactions.
	protocolTimeout := time.After(bcConfig.BlockInterval / 2)

	var txs []ClientTransaction
	commonVersion := Version(0)

collectTxLoop:
	for {
		select {
		case commonVersion = <-root.CommonVersionChan:
			// The value gives a version that is the same for a threshold of conodes but it
			// can be the latest version available so it needs to check that to not create a
			// block to upgrade from version x to x (which is not an upgrade per se).
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
			break collectTxLoop
		}
	}

	return &collectTxResult{Txs: txs, CommonVersion: commonVersion}, nil
}

func (s *defaultTxProcessor) ProcessTx(tx ClientTransaction, inState *txProcessorState) ([]*txProcessorState, error) {
	s.Lock()
	latest := s.latest
	s.Unlock()
	if latest == nil {
		return nil, xerrors.New("missing latest block in processor")
	}

	header, err := decodeBlockHeader(latest)
	if err != nil {
		return nil, xerrors.Errorf("decoding header: %v", err)
	}

	tx.Instructions.SetVersion(header.Version)

	scsOut, sstOut, err := s.processOneTx(inState.sst, tx, s.scID, header.Timestamp)

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
	config, err := state.sst.LoadConfigFromTrie()
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

func (s *defaultTxProcessor) GetInterval() time.Duration {
	bcConfig, err := s.LoadConfig(s.scID)
	if err != nil {
		log.Error(s.ServerIdentity(), "couldn't get configuration - this is bad and probably "+
			"a problem with the database! ", err)
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
			"a problem with the database! ", err)
		return defaultMaxBlockSize
	}
	return bcConfig.MaxBlockSize
}

func (s *defaultTxProcessor) Stop() {
	close(s.stopCollect)
}

type txPipeline struct {
	ctxChan     chan ClientTransaction
	needUpgrade chan Version
	stopCollect chan bool
	wg          sync.WaitGroup
	processor   txProcessor
}

func (p *txPipeline) start(initialState *txProcessorState, stopSignal chan bool) {
	p.stopCollect = make(chan bool)
	p.ctxChan = make(chan ClientTransaction, 200)
	p.needUpgrade = make(chan Version, 1)

	p.collectTx()
	p.processTxs(initialState)

	<-stopSignal
	close(p.stopCollect)
	p.processor.Stop()
	p.wg.Wait()
}

func (p *txPipeline) collectTx() {
	p.wg.Add(1)

	// set the polling interval to half of the block interval
	go func() {
		defer p.wg.Done()
		for {
			interval := p.processor.GetInterval()
			select {
			case <-p.stopCollect:
				log.Lvl3("stopping tx collector")
				close(p.ctxChan)
				return
			case <-time.After(interval / 2):
				res, err := p.processor.CollectTx()
				if err != nil {
					log.Error("failed to collect transactions", err)
				}

				// If a common version is found, it is sent anyway and the processing
				// will check if it is necessary to upgrade.
				p.needUpgrade <- res.CommonVersion

				for _, tx := range res.Txs {
					select {
					case p.ctxChan <- tx:
						// channel not full, do nothing
					default:
						log.Warn("dropping transactions because there are too many")
					}
				}
			}
		}
	}()
}

var maxTxHashes = 1000

// processTxs consumes transactions and computes the new txResults
func (p *txPipeline) processTxs(initialState *txProcessorState) {
	var proposing bool
	// always use the latest one when adding new
	currentState := []*txProcessorState{initialState}
	currentVersion := p.processor.GetLatestGoodState().sst.GetVersion()
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
			case version := <-p.needUpgrade:
				if version <= currentVersion {
					// Prevent multiple upgrade blocks for the same version.
					break
				}

				// An upgrade is done synchronously so that other operations
				// are not performed until the upgrade is done.

				if proposing {
					// If a block is currently created, it will wait for the
					// end of the process.
					err := <-proposalResult
					proposalResult <- err
				}

				err := p.processor.ProposeUpgradeBlock(version)
				if err != nil {
					// Only log the error as it won't prevent normal blocks
					// to be created.
					log.Error("failed to upgrade", err)
				}

				currentVersion = version
			case <-intervalChan:
				// update the interval every time because it might've changed
				intervalChan = getInterval()

				if proposing {
					// Wait for the end of the block creation to prevent too many transactions
					// to be processed and thus makes an even longer block after the new one.
					err := <-proposalResult
					// Only the ProposeBlock sends back results and it sends only one
					proposing = false
					if err != nil {
						log.Error("reverting to last known state because proposal refused:", err)
						currentState = []*txProcessorState{p.processor.GetLatestGoodState()}
						break
					}
				}

				// wait for the next interval if there are no changes
				// we do not check for the length because currentState
				// should always be non-empty, otherwise it's a
				// programmer error
				if len(currentState[0].txs) == 0 {
					break
				}

				proposing = true

				// find the right state and propose it in the block
				var inState *txProcessorState
				currentState, inState = proposeInputState(currentState)

				go func(state *txProcessorState) {
					p.wg.Add(1)
					defer p.wg.Done()
					if state != nil {
						// NOTE: ProposeBlock might block for a long time,
						// but there's nothing we can do about it at the moment
						// other than waiting for the timeout.
						err := p.processor.ProposeBlock(state)
						if err != nil {
							log.Error("failed to propose block:", err)
							proposalResult <- err
							return
						}
					}
					proposalResult <- nil
				}(inState)
			case tx, ok := <-p.ctxChan:
				select {
				// This case has a higher priority so we force the select to go through it
				// first.
				case <-intervalChan:
					intervalChan = time.After(0)
					break
				default:
				}

				if !ok {
					log.Lvl3("stopping txs processor")
					return
				}
				txh := tx.Instructions.HashWithSignatures()
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
					log.Error("processing transaction failed with error:", err)
				} else {
					// Remove the last one from currentState because
					// it might be getting updated and then append newStates.
					currentState = append(currentState[:len(currentState)-1], newStates...)
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
