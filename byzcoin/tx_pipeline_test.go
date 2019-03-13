package byzcoin

import (
	"bytes"
	"errors"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/darc"

	"testing"
	"time"
)

type mockTxProcessor struct {
	batch        int
	txCtr        int
	proposed     TxResults
	failAt       int                 // instructs ProposeBlock to return an error when processing the tx at the failAt index
	txs          []ClientTransaction // we assume these are unique
	done         chan bool
	proposeDelay time.Duration
	collectDelay time.Duration
	goodState    *stagingStateTrie
	t            *testing.T
}

func (p *mockTxProcessor) CollectTx() ([]ClientTransaction, error) {
	time.Sleep(p.collectDelay) // simulate slow network/protocol
	if p.txCtr + p.batch > len(p.txs) {
		return nil, nil
	}
	out := p.txs[p.txCtr : p.txCtr + p.batch]
	p.txCtr += p.batch
	return out, nil
}

func txEqual(a, b ClientTransaction) bool {
	return bytes.Equal(a.Instructions.Hash(), b.Instructions.Hash())
}

func (p *mockTxProcessor) ProcessTx(tx ClientTransaction, inState *txProcessorState) (*txProcessorState, error) {
	sc := StateChange {
		StateAction: Create,
		InstanceID: tx.Instructions.Hash(),
		ContractID: "",
		Value: tx.Instructions.Hash(),
		DarcID: darc.ID{},
		Version: 0,
	}
	if err := inState.sst.Set(sc.Key(), sc.Val()); err != nil {
		return nil, err
	}
	return &txProcessorState{
		sst: inState.sst,
		coins: []Coin{},
		scs: append(inState.scs, sc),
		txs: append(inState.txs, TxResult{tx, true}),
	}, nil
}

func (p *mockTxProcessor) ProposeBlock(state *txProcessorState) error {
	// simulate slow consensus
	time.Sleep(p.proposeDelay)

	// simulate failure
	if len(p.proposed) <= p.failAt && p.failAt < len(p.proposed) + len(state.txs) {
		// we only fail it once, so reset it here
		p.failAt = len(p.txs)
		return errors.New("simulating proposal failure")
	}

	p.proposed = append(p.proposed, state.txs...)
	p.goodState = state.sst
	if len(p.proposed) == len(p.txs) {
		require.True(p.t, txEqual(state.txs[len(state.txs)-1].ClientTransaction, p.txs[len(p.txs)-1]))
		close(p.done)
	}
	return nil
}

func (p *mockTxProcessor) GetInterval() (time.Duration, error) {
	return 100 * time.Millisecond, nil
}

func (p *mockTxProcessor) Stop() {
}

func (p *mockTxProcessor) GetLatestGoodState() (*txProcessorState, error) {
	goodState := p.goodState
	if goodState == nil {
		sst, err := newMemStagingStateTrie([]byte(""))
		require.NoError(p.t, err)
		goodState = sst
	}
	return &txProcessorState{
		 sst: goodState,
	}, nil
}

func TestTxPipeline(t *testing.T) {
	testTxPipeline(t, 1, 1, 1)
	testTxPipeline(t, 4, 1, 4)
	testTxPipeline(t, 4, 2, 4)
}

func TestTxPipelineFailure(t *testing.T) {
	testTxPipeline(t, 8, 2, 1)
	testTxPipeline(t, 8, 2, 2)
}

func testTxPipeline(t *testing.T, n, batch, failAt int) {
	txs := make([]ClientTransaction, n)
	for i := range txs {
		txs[i].Instructions = []Instruction{
			{
				InstanceID: NewInstanceID([]byte{byte(i)}),
				Invoke: &Invoke{
					ContractID: "",
					Command: string(i),
				},
			},
		}
	}

	processor := mockTxProcessor{
		batch:  batch,
		txs:    txs,
		done:   make(chan bool, 1),
		t:      t,
		failAt: failAt,
	}

	pipeline := txPipeline{
		processor: &processor,
	}
	sst, err := newMemStagingStateTrie([]byte(""))
	require.NoError(t, err)
	stopChan := pipeline.start(&txProcessorState{
		sst: sst,
	})

	interval, err := processor.GetInterval()
	require.NoError(t, err)

	if failAt < n {
		// we should not hear from processor.done
		select {
		case <-processor.done:
			require.Fail(t, "block proposal should have failed")
		case <- time.After(5 * time.Duration(n/batch) * interval):
		}
		// the list of proposed transactions should have some missing blocks
		require.True(t, len(processor.proposed) > 0)
		require.True(t, len(processor.proposed) < len(txs))
	} else {
		// done chan should be closed after all txs are processed
		select {
		case <-processor.done:
		case <- time.After(5 * time.Duration(n/batch) * interval):
			require.Fail(t, "tx processor did not finish in time")
		}
	}

	close(stopChan)

}
