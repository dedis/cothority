package byzcoin

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/darc"

	"testing"
	"time"
)

type mockTxProc interface {
	txProcessor
	Done() chan bool
	GetProposed() TxResults
}

type defaultMockTxProc struct {
	sync.Mutex
	batch            int
	txCtr            int
	proposed         TxResults
	failAt           int                 // instructs ProposeBlock to return an error when processing the tx at the failAt index
	txs              []ClientTransaction // we assume these are unique
	commonVersion    Version
	stateTrieVersion Version
	done             chan bool
	proposeDelay     time.Duration
	collectDelay     time.Duration
	goodState        *stagingStateTrie
	t                *testing.T
}

func txEqual(a, b ClientTransaction) bool {
	return bytes.Equal(a.Instructions.Hash(), b.Instructions.Hash())
}

func (p *defaultMockTxProc) CollectTx() (*collectTxResult, error) {
	p.Lock()
	defer p.Unlock()

	time.Sleep(p.collectDelay) // simulate slow network/protocol
	if p.txCtr+p.batch > len(p.txs) {
		return &collectTxResult{}, nil
	}
	out := p.txs[p.txCtr : p.txCtr+p.batch]
	p.txCtr += p.batch
	return &collectTxResult{Txs: out, CommonVersion: p.commonVersion}, nil
}

func (p *defaultMockTxProc) ProcessTx(tx ClientTransaction, inState *txProcessorState) ([]*txProcessorState, error) {
	sc := StateChange{
		StateAction: Create,
		InstanceID:  tx.Instructions.Hash(),
		ContractID:  "",
		Value:       tx.Instructions.Hash(),
		DarcID:      darc.ID{},
		Version:     0,
	}
	if err := inState.sst.Set(sc.Key(), sc.Val()); err != nil {
		return nil, err
	}
	return []*txProcessorState{{
		sst: inState.sst,
		scs: append(inState.scs, sc),
		txs: append(inState.txs, TxResult{tx, true}),
	}}, nil
}

func (p *defaultMockTxProc) ProposeBlock(state *txProcessorState) error {
	// simulate slow consensus
	time.Sleep(p.proposeDelay)

	require.True(p.t, len(state.txs) > 0)
	require.True(p.t, len(state.scs) > 0)

	p.Lock()
	defer p.Unlock()

	// simulate failure
	if len(p.proposed) <= p.failAt && p.failAt < len(p.proposed)+len(state.txs) {
		// we only fail it once, so reset it here
		p.failAt = len(p.txs)
		return errors.New("simulating proposal failure")
	}

	p.proposed = append(p.proposed, state.txs...)
	p.goodState = state.sst
	if len(p.proposed) == len(p.txs) {
		require.True(p.t, txEqual(state.txs[len(state.txs)-1].ClientTransaction, p.txs[len(p.txs)-1]))

		// check the final state by replaying all the transactions
		root, err := replayMockTxs(p.txs)
		require.NoError(p.t, err)
		require.Equal(p.t, root, state.sst.GetRoot())

		close(p.done)
	}
	return nil
}

func (p *defaultMockTxProc) ProposeUpgradeBlock(version Version) error {
	p.Lock()
	defer p.Unlock()

	require.True(p.t, p.commonVersion > p.stateTrieVersion)
	p.stateTrieVersion = p.commonVersion
	p.goodState = nil
	return nil
}

func (p *defaultMockTxProc) GetInterval() time.Duration {
	return 100 * time.Millisecond
}

func (p *defaultMockTxProc) Stop() {
}

func (p *defaultMockTxProc) GetLatestGoodState() *txProcessorState {
	goodState := p.goodState
	if goodState == nil {
		p.Lock()
		defer p.Unlock()

		sst, err := newMemStagingStateTrie([]byte(""))
		require.NoError(p.t, err)

		versionBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(versionBuf, uint32(p.stateTrieVersion))
		sst.Set([]byte(trieVersionKey), versionBuf)
		goodState = sst
	}
	return &txProcessorState{
		sst: goodState,
	}
}

func (p *defaultMockTxProc) GetBlockSize() int {
	return 1e3
}

func (p *defaultMockTxProc) Done() chan bool {
	return p.done
}

func (p *defaultMockTxProc) GetProposed() TxResults {
	p.Lock()
	defer p.Unlock()

	return p.proposed
}

type newMockTxProcFunc func(t *testing.T, batch int, txs []ClientTransaction, failAt int) mockTxProc

func newDefaultMockTxProc(t *testing.T, batch int, txs []ClientTransaction, failAt int) mockTxProc {
	proc := &defaultMockTxProc{
		batch:        batch,
		txs:          txs,
		done:         make(chan bool, 1),
		t:            t,
		failAt:       failAt,
		proposeDelay: 10 * time.Microsecond,
		collectDelay: 10 * time.Microsecond,
	}
	proc.proposeDelay = proc.GetInterval() / 10
	proc.collectDelay = proc.GetInterval() / 10
	return proc
}

func newSlowBlockMockTxProc(t *testing.T, batch int, txs []ClientTransaction, failAt int) mockTxProc {
	proc := &defaultMockTxProc{
		batch:        batch,
		txs:          txs,
		done:         make(chan bool, 1),
		t:            t,
		failAt:       failAt,
		proposeDelay: 10 * time.Microsecond,
		collectDelay: 10 * time.Microsecond,
	}
	proc.proposeDelay = proc.GetInterval() + proc.GetInterval()/10
	proc.collectDelay = proc.GetInterval() / 10
	return proc
}

func newSlowCollectMockTxProc(t *testing.T, batch int, txs []ClientTransaction, failAt int) mockTxProc {
	proc := &defaultMockTxProc{
		batch:        batch,
		txs:          txs,
		done:         make(chan bool, 1),
		t:            t,
		failAt:       failAt,
		proposeDelay: 10 * time.Microsecond,
		collectDelay: 10 * time.Microsecond,
	}
	proc.proposeDelay = proc.GetInterval() / 10
	proc.collectDelay = proc.GetInterval() * 2
	return proc
}

func TestTxPipeline(t *testing.T) {
	testTxPipeline(t, 1, 1, 1, newDefaultMockTxProc)
	testTxPipeline(t, 4, 1, 4, newDefaultMockTxProc)
	testTxPipeline(t, 8, 2, 8, newDefaultMockTxProc)
}

func TestTxPipeline_Failure(t *testing.T) {
	testTxPipeline(t, 8, 2, 1, newDefaultMockTxProc)
	testTxPipeline(t, 8, 2, 2, newDefaultMockTxProc)
}

func TestTxPipeline_Slow(t *testing.T) {
	testTxPipeline(t, 4, 1, 4, newSlowBlockMockTxProc)
	testTxPipeline(t, 4, 1, 4, newSlowCollectMockTxProc)
}

func replayMockTxs(txs []ClientTransaction) ([]byte, error) {
	sst, err := newMemStagingStateTrie([]byte(""))
	if err != nil {
		return nil, err
	}

	for _, tx := range txs {
		sc := StateChange{
			StateAction: Create,
			InstanceID:  tx.Instructions.Hash(),
			ContractID:  "",
			Value:       tx.Instructions.Hash(),
			DarcID:      darc.ID{},
			Version:     0,
		}
		if err := sst.Set(sc.Key(), sc.Val()); err != nil {
			return nil, err
		}
	}
	return sst.GetRoot(), nil
}

func testTxPipeline(t *testing.T, n, batch, failAt int, mock newMockTxProcFunc) {
	txs := make([]ClientTransaction, n)
	for i := range txs {
		txs[i].Instructions = []Instruction{
			{
				InstanceID: NewInstanceID([]byte{byte(i)}),
				Invoke: &Invoke{
					ContractID: "",
					Command:    string(i),
				},
			},
		}
	}

	processor := mock(t, batch, txs, failAt)
	pipeline := txPipeline{
		processor: processor.(txProcessor),
	}
	sst, err := newMemStagingStateTrie([]byte(""))
	require.NoError(t, err)
	stopChan := make(chan bool)
	pipelineDone := make(chan bool)
	go func() {
		pipeline.start(&txProcessorState{
			sst: sst,
		}, stopChan)
		close(pipelineDone)
	}()

	mockproc, ok := processor.(*defaultMockTxProc)
	if ok {
		<-time.After(mockproc.collectDelay)
		mockproc.Lock()
		mockproc.commonVersion = 3
		mockproc.Unlock()
	}

	interval := processor.GetInterval()

	if failAt < n {
		// we should not hear from processor.done
		select {
		case <-processor.Done():
			require.Fail(t, "block proposal should have failed")
		case <-time.After(5 * time.Duration(n/batch) * interval):
		}
		// the list of proposed transactions should have some missing blocks
		require.True(t, len(processor.GetProposed()) > 0)
		require.True(t, len(processor.GetProposed()) < len(txs))
	} else {
		// done chan should be closed after all txs are processed
		select {
		case <-processor.Done():
		case <-time.After(5 * time.Duration(n/batch) * interval):
			require.Fail(t, "tx processor did not finish in time")
		}
	}

	// Check if the version is up-to-date with the latest.
	if ok {
		require.Equal(t, mockproc.commonVersion, mockproc.stateTrieVersion)
	}

	close(stopChan)

	select {
	case <-pipelineDone:
	case <-time.After(time.Second):
		require.Fail(t, "pipeline.start should have returned")
	}
}

type bigMockTxProc struct {
	*defaultMockTxProc
}

// ProcessTx will produce two states when the input state has more than 1
// transaction.
func (p *bigMockTxProc) ProcessTx(tx ClientTransaction, inState *txProcessorState) ([]*txProcessorState, error) {
	sc := StateChange{
		StateAction: Create,
		InstanceID:  tx.Instructions.Hash(),
		ContractID:  "",
		Value:       tx.Instructions.Hash(),
		DarcID:      darc.ID{},
		Version:     0,
	}
	newState := inState.sst.Clone()
	if err := newState.Set(sc.Key(), sc.Val()); err != nil {
		return nil, err
	}
	if len(inState.txs) > 0 {
		return []*txProcessorState{
			// keep the old one as it was
			inState,
			// the new state doesn't include the old scs or txs
			{
				newState,
				[]StateChange{sc},
				[]TxResult{{tx, true}},
				0,
			},
		}, nil
	}
	return []*txProcessorState{{
		newState,
		append(inState.scs, sc),
		append(inState.txs, TxResult{tx, true}),
		0,
	}}, nil
}

func newBigMockTxProc(t *testing.T, batch int, txs []ClientTransaction, failAt int) mockTxProc {
	proc := newDefaultMockTxProc(t, batch, txs, failAt).(*defaultMockTxProc)
	return &bigMockTxProc{
		defaultMockTxProc: proc,
	}
}

// TestTxPipeline_BigTx tests the situation when ProcessTx returns more than
// one state. This event happens when the state becomes too big to fit into one
// block so it will "overflow" into a new state. In this case we should get two
// blocks.
func TestTxPipeline_BigTx(t *testing.T) {
	testTxPipeline(t, 4, 1, 4, newBigMockTxProc)
	testTxPipeline(t, 8, 2, 8, newBigMockTxProc)
}
