package byzcoin

import (
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
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
	SignerCounter       uint64
	PropagationInterval time.Duration
	Client              *Client
	GenesisMessage      *CreateGenesisBlock
	T                   *testing.T
}

type BCTestArgs struct {
	PropagationInterval time.Duration
	Nodes               int
}

type TxArgs struct {
	Node            int
	Wait            int
	WaitPropagation bool
	RequireSuccess  bool
}

var TxArgsDefault = TxArgs{
	Node:            0,
	Wait:            10,
	WaitPropagation: true,
	RequireSuccess:  true,
}

var defaultBCTestArgs = BCTestArgs{
	PropagationInterval: 500 * time.Millisecond,
	Nodes:               3,
}

// NewBCTestWithArgs takes a BCTestArgs to override the default values.
func NewBCTest(t *testing.T, ba *BCTestArgs) *BCTest {
	if ba == nil {
		ba = &defaultBCTestArgs
	}
	b := &BCTest{
		T:             t,
		Local:         onet.NewLocalTestT(tSuite, t),
		Value:         []byte("anyvalue"),
		Signer:        darc.NewSignerEd25519(nil, nil),
		SignerCounter: 1,
	}

	require.NoError(t, RegisterGlobalContract(DummyContractName,
		DummyContractFromBytes))
	b.Servers, b.Roster, _ = b.Local.GenTree(ba.Nodes, true)
	for _, sv := range b.Local.GetServices(b.Servers, ByzCoinID) {
		service := sv.(*Service)
		b.Services = append(b.Services, service)
	}

	var err error
	b.GenesisMessage, err = DefaultGenesisMsg(CurrentVersion, b.Roster, nil,
		b.Signer.Identity())
	require.NoError(t, err)
	b.GenesisDarc = &b.GenesisMessage.GenesisDarc

	b.GenesisMessage.BlockInterval = ba.PropagationInterval
	b.PropagationInterval = b.GenesisMessage.BlockInterval

	return b
}

func (b *BCTest) AddGenesisRules(rules ...string) {
	ownerExpr := expression.Expr(b.Signer.Identity().String())
	for _, r := range rules {
		require.NoError(b.T, b.GenesisMessage.GenesisDarc.Rules.AddRule(
			darc.Action(r), ownerExpr))
	}
}

func (b *BCTest) CreateByzCoin() {
	resp, err := b.Services[0].CreateGenesisBlock(b.GenesisMessage)
	require.NoError(b.T, err)
	b.Genesis = resp.Skipblock
	b.Client = NewClient(b.Genesis.SkipChainID(), *b.Roster)
	require.NoError(b.T, b.Client.WaitPropagation(0))
}

func (b *BCTest) NodeStop(index int) {
	b.Services[index].TestClose()
	b.Servers[index].Pause()
}

func (b *BCTest) NodeRestart(index int) {
	b.Servers[index].Unpause()
	require.NoError(b.T, b.Services[index].TestRestart())
}

func (b *BCTest) CloseAll() {
	if b.Client != nil {
		require.NoError(b.T, b.Client.WaitPropagation(-1))
	}
	b.Local.CloseAll()
}

func (b *BCTest) SendInst(args *TxArgs,
	inst ...Instruction) (ClientTransaction, AddTxResponse) {

	for i := range inst {
		inst[i].SignerCounter = []uint64{b.SignerCounter}
		b.SignerCounter++
	}
	ctx, err := combineInstrsAndSign(b.Signer, inst...)
	require.NoError(b.T, err)

	return ctx, b.SendTx(args, ctx)
}

func (b *BCTest) SendTx(args *TxArgs, ctx ClientTransaction) AddTxResponse {
	if args == nil {
		args = &TxArgsDefault
	}

	resp, err := b.Services[args.Node].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   b.Genesis.SkipChainID(),
		Transaction:   ctx,
		InclusionWait: args.Wait,
	})
	require.NoError(b.T, err)
	require.NotNil(b.T, resp)
	if args.RequireSuccess {
		require.Empty(b.T, resp.Error)
	}
	if args.WaitPropagation {
		require.NoError(b.T, b.Client.WaitPropagation(-1))
	}
	return *resp
}

func (b *BCTest) SpawnDummy(args *TxArgs) (ClientTransaction, AddTxResponse) {
	return b.SendInst(args, Instruction{
		InstanceID: NewInstanceID(b.GenesisDarc.GetBaseID()),
		Spawn: &Spawn{
			ContractID: DummyContractName,
			Args:       Arguments{{Name: "data", Value: []byte("anyvalue")}},
		},
	})
}

func (b *BCTest) DeleteDBs(index int) {
	bc := b.Services[index]
	log.Lvlf1("%s: Deleting DB of node %d", bc.ServerIdentity(), index)
	bc.TestClose()
	for scid := range bc.stateTries {
		require.NoError(b.T, deleteDB(bc.ServiceProcessor, []byte(scid)))
		idStr := hex.EncodeToString([]byte(scid))
		require.NoError(b.T, deleteDB(bc.ServiceProcessor, []byte(idStr)))
	}
	require.NoError(b.T, deleteDB(bc.ServiceProcessor, storageID))
	sc := bc.Service(skipchain.ServiceName).(*skipchain.Service)
	require.NoError(b.T, deleteDB(sc.ServiceProcessor, []byte("skipblocks")))
	require.NoError(b.T, deleteDB(sc.ServiceProcessor, []byte("skipchainconfig")))
	require.NoError(b.T, bc.TestRestart())
}

func deleteDB(s *onet.ServiceProcessor, key []byte) error {
	db, stBucket := s.GetAdditionalBucket(key)
	return db.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket(stBucket)
	})
}

const DummyContractName = "dummy"

type DummyContract struct {
	BasicContract
	Data []byte
}

func DummyContractFromBytes(in []byte) (Contract, error) {
	return &DummyContract{Data: in}, nil
}

func (dc *DummyContract) Spawn(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	_, _, _, darcID, err := cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	if len(inst.Spawn.Args.Search("data")) == 32 {
		return []StateChange{
			NewStateChange(Create, NewInstanceID(inst.Spawn.Args.Search("data")), inst.Spawn.ContractID,
				[]byte{}, darcID),
		}, nil, nil
	}
	return []StateChange{
		NewStateChange(Create, NewInstanceID(inst.Hash()), inst.Spawn.ContractID, inst.Spawn.Args[0].Value, darcID),
	}, nil, nil
}

func (dc *DummyContract) Invoke(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	_, _, _, darcID, err := cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	return []StateChange{
		NewStateChange(Update, inst.InstanceID, DummyContractName, inst.Invoke.Args[0].Value, darcID),
	}, nil, nil
}

func (dc *DummyContract) Delete(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	_, _, _, darcID, err := cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	return []StateChange{
		NewStateChange(Remove, inst.InstanceID, "", nil, darcID),
	}, nil, nil
}
