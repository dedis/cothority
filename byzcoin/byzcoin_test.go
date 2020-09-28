package byzcoin

import (
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.etcd.io/bbolt"
	"testing"
	"time"
)

type BCTest struct {
	Local               *onet.LocalTest
	Servers             []*onet.Server
	Roster              *onet.Roster
	Services            []*Service
	Genesis             *skipchain.SkipBlock
	Value               []byte
	GenesisDarc         *darc.Darc
	Signer              darc.Signer
	CTx                 ClientTransaction
	PropagationInterval time.Duration
	Client              *Client
	t                   *testing.T
}

type BCTestArgs struct {
	Step                int
	PropagationInterval time.Duration
	Nodes               int
	RotationWindow      int
	Version             Version
}

// NewBCTestArgs returns a default BCTestArgs structure.
// The values in here guarantee a fast but still passable test in travis.
func NewBCTestArgs() BCTestArgs {
	return BCTestArgs{1,
		500 * time.Millisecond,
		3,
		// use this value as a rotation window to make it impossible to trigger a view change
		9999,
		CurrentVersion}
}

// NewBCTest returns a default and initialized BCTest structure.
// It already started a testing byzcoin instance.
func NewBCTest(t *testing.T) *BCTest {
	return NewBCTestWithArgs(t, NewBCTestArgs())
}

// NewBCTestWithArgs takes a BCTestArgs to override the default values.
func NewBCTestWithArgs(t *testing.T, ba BCTestArgs) *BCTest {
	b := &BCTest{
		t:      t,
		Local:  onet.NewLocalTestT(tSuite, t),
		Value:  []byte("anyvalue"),
		Signer: darc.NewSignerEd25519(nil, nil),
	}
	b.Servers, b.Roster, _ = b.Local.GenTree(ba.Nodes, true)
	for _, sv := range b.Local.GetServices(b.Servers, ByzCoinID) {
		service := sv.(*Service)
		service.rotationWindow = ba.RotationWindow
		service.defaultVersion = ba.Version
		b.Services = append(b.Services, service)
	}
	registerDummy(t, b.Servers)

	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, b.Roster,
		[]string{
			"spawn:" + dummyContract,
			"spawn:" + invalidContract,
			"spawn:" + panicContract,
			"spawn:" + slowContract,
			"spawn:" + versionContract,
			"spawn:" + stateChangeCacheContract,
			"delete:" + dummyContract,
		}, b.Signer.Identity())
	require.NoError(t, err)
	b.GenesisDarc = &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = ba.PropagationInterval
	b.PropagationInterval = genesisMsg.BlockInterval

	for i := 0; i < ba.Step; i++ {
		switch i {
		case 0:
			resp, err := b.Service().CreateGenesisBlock(genesisMsg)
			require.NoError(t, err)
			b.Genesis = resp.Skipblock
			b.WaitPropagation(0)
			b.Client = NewClient(b.Genesis.SkipChainID(), *b.Roster)
		case 1:
			tx, err := createOneClientTx(b.GenesisDarc.GetBaseID(), dummyContract, b.Value, b.Signer)
			require.NoError(t, err)
			b.CTx = tx
			resp, err := b.Service().AddTransaction(&AddTxRequest{
				Version:       CurrentVersion,
				SkipchainID:   b.Genesis.SkipChainID(),
				Transaction:   tx,
				InclusionWait: 10,
			})
			transactionOK(t, resp, err)
			b.WaitPropagation(1)
		default:
			require.Fail(t, "no such step")
		}
	}
	return b
}

func (b *BCTest) CloseAll() {
	b.WaitPropagation(-1)
	b.Local.CloseAll()
}

func (b *BCTest) Service() *Service {
	return b.Services[0]
}

func (b *BCTest) WaitProof(id InstanceID) Proof {
	return b.WaitProofWithIdx(id.Slice(), 0)
}

func (b *BCTest) WaitProofWithIdx(key []byte, idx int) Proof {
	var pr Proof
	var ok bool
	for i := 0; i < 10; i++ {
		resp, err := b.Services[idx].GetProof(&GetProof{
			Version: CurrentVersion,
			Key:     key,
			ID:      b.Genesis.SkipChainID(),
		})
		if err == nil {
			pr = resp.Proof
			if pr.InclusionProof.Match(key) {
				ok = true
				break
			}
		}

		// wait for the block to be processed
		time.Sleep(2 * b.PropagationInterval)
	}

	require.True(b.t, ok, "got not match")
	return pr
}

func (b *BCTest) SendTx(ctx ClientTransaction) {
	b.SendTxTo(ctx, 0)
}

func (b *BCTest) SendTxTo(ctx ClientTransaction, idx int) {
	b.SendTxToAndWait(ctx, idx, 0)
}

func (b *BCTest) SendTxWaitPropagation(ctx ClientTransaction, idx int) {
	b.SendTxToAndWait(ctx, idx, 20)
	b.WaitPropagation(-1)
}

func (b *BCTest) SendTxAndWait(ctx ClientTransaction, wait int) {
	b.SendTxToAndWait(ctx, 0, wait)
}

func (b *BCTest) SendTxToAndWait(ctx ClientTransaction, idx int, wait int) {
	resp, err := b.Services[idx].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   b.Genesis.SkipChainID(),
		Transaction:   ctx,
		InclusionWait: wait,
	})
	transactionOK(b.t, resp, err)
}

func (b *BCTest) SendDummyTx(node int, counter uint64, wait int) {
	tx1, err := createOneClientTxWithCounter(b.GenesisDarc.GetBaseID(),
		dummyContract, b.Value, b.Signer, counter)
	require.NoError(b.t, err)
	b.SendTxToAndWait(tx1, node, wait)
}

func (b *BCTest) SendDummyTxWaitPropagation(node int, counter uint64) {
	b.SendDummyTx(node, counter, 20)
	b.WaitPropagation(-1)
}

// caller gives us a darc, and we try to make an evolution request.
func (b *BCTest) TestDarcEvolution(d2 darc.Darc, fail bool) (pr *Proof) {
	counterResponse, err := b.Service().GetSignerCounters(&GetSignerCounters{
		SignerIDs:   []string{b.Signer.Identity().String()},
		SkipchainID: b.Genesis.SkipChainID(),
	})
	require.NoError(b.t, err)

	ctx := b.DarcToTx(d2, counterResponse.Counters[0]+1)
	b.SendTx(ctx)
	for i := 0; i < 10; i++ {
		resp, err := b.Service().GetProof(&GetProof{
			Version: CurrentVersion,
			Key:     d2.GetBaseID(),
			ID:      b.Genesis.SkipChainID(),
		})
		require.NoError(b.t, err)
		pr = &resp.Proof
		_, v0, _, _, err := pr.KeyValue()
		require.NoError(b.t, err)
		d, err := darc.NewFromProtobuf(v0)
		require.NoError(b.t, err)
		if d.Equal(&d2) {
			return
		}
		time.Sleep(b.PropagationInterval)
	}
	if !fail {
		b.t.Fatal("couldn't store new darc")
	}
	return
}

func (b *BCTest) DeleteDBs(index int) {
	bc := b.Services[index]
	log.Lvlf1("%s: Deleting DB of node %d", bc.ServerIdentity(), index)
	bc.TestClose()
	for scid := range bc.stateTries {
		require.NoError(b.t, deleteDB(bc.ServiceProcessor, []byte(scid)))
		idStr := hex.EncodeToString([]byte(scid))
		require.NoError(b.t, deleteDB(bc.ServiceProcessor, []byte(idStr)))
	}
	require.NoError(b.t, deleteDB(bc.ServiceProcessor, storageID))
	sc := bc.Service(skipchain.ServiceName).(*skipchain.Service)
	require.NoError(b.t, deleteDB(sc.ServiceProcessor, []byte("skipblocks")))
	require.NoError(b.t, deleteDB(sc.ServiceProcessor, []byte("skipchainconfig")))
	require.NoError(b.t, bc.TestRestart())
}

// Waits to have a coherent view in all nodes with at least the block
// 'index' held by all nodes.
func (b *BCTest) WaitPropagation(index int) {
	if b.Genesis != nil && b.Roster != nil {
		require.NoError(b.t, NewClient(b.Genesis.Hash,
			*b.Roster).WaitPropagation(index))
	}
}

func (b *BCTest) DarcToTx(d2 darc.Darc, ctr uint64) ClientTransaction {
	d2Buf, err := d2.ToProto()
	require.NoError(b.t, err)
	invoke := Invoke{
		ContractID: ContractDarcID,
		Command:    cmdDarcEvolve,
		Args: []Argument{
			{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}
	instr := Instruction{
		InstanceID:    NewInstanceID(d2.GetBaseID()),
		Invoke:        &invoke,
		SignerCounter: []uint64{ctr},
		version:       CurrentVersion,
	}
	ctx, err := combineInstrsAndSign(b.Signer, instr)
	require.NoError(b.t, err)
	return ctx
}

func deleteDB(s *onet.ServiceProcessor, key []byte) error {
	db, stBucket := s.GetAdditionalBucket(key)
	return db.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket(stBucket)
	})
}

func transactionOK(t *testing.T, resp *AddTxResponse, err error) {
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.Error)
}
