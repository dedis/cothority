package byzcoin

import (
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3"
	"testing"
	"time"
)

// BCTest structure represents commonly used elements when doing integration
// tests in ByzCoin.
// The different methods use the testing.T field
// to reduce manual error checking.
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

// NewBCTestDefault creates a new structure with default values.
func NewBCTestDefault(t *testing.T) *BCTest {
	return NewBCTest(t, 500*time.Millisecond, 3)
}

// NewBCTest creates a new structure for the test, initializing the nodes,
// but not yet starting the byzcoin instance.
func NewBCTest(t *testing.T, propagation time.Duration, nodes int) *BCTest {
	b := &BCTest{
		T:             t,
		Local:         onet.NewLocalTestT(suites.MustFind("Ed25519"), t),
		Value:         []byte("anyvalue"),
		Signer:        darc.NewSignerEd25519(nil, nil),
		SignerCounter: 1,
	}

	b.Servers, b.Roster, _ = b.Local.GenTree(nodes, true)
	for _, sv := range b.Local.GetServices(b.Servers, ByzCoinID) {
		service := sv.(*Service)
		b.Services = append(b.Services, service)
	}

	var err error
	b.GenesisMessage, err = DefaultGenesisMsg(CurrentVersion, b.Roster,
		[]string{
			"spawn:" + DummyContractName,
			"delete:" + DummyContractName,
		},
		b.Signer.Identity())
	require.NoError(t, err)
	b.GenesisDarc = &b.GenesisMessage.GenesisDarc

	b.GenesisMessage.BlockInterval = propagation
	b.PropagationInterval = b.GenesisMessage.BlockInterval

	return b
}

// AddGenesisRules can be used before CreateByzCoin to add rules to the
// genesis darc.
func (b *BCTest) AddGenesisRules(rules ...string) {
	require.Nil(b.T, b.Genesis, "cannot add rules after CreateByzCoin")
	ownerExpr := expression.Expr(b.Signer.Identity().String())
	for _, r := range rules {
		require.NoError(b.T, b.GenesisMessage.GenesisDarc.Rules.AddRule(
			darc.Action(r), ownerExpr))
	}
}

// CreateByzCoin starts the byzcoin instance and updates the Genesis and
// Client fields.
func (b *BCTest) CreateByzCoin() {
	require.Nil(b.T, b.Genesis, "CreateByzCoin can only be called once")
	resp, err := b.Services[0].CreateGenesisBlock(b.GenesisMessage)
	require.NoError(b.T, err)
	b.Genesis = resp.Skipblock
	b.Client = NewClient(b.Genesis.SkipChainID(), *b.Roster)
	require.NoError(b.T, b.Client.WaitPropagation(0))
}

// NodeStop simulates a node that is down.
func (b *BCTest) NodeStop(index int) {
	b.Services[index].TestClose()
	b.Servers[index].Pause()
}

// NodeRestart simulates a node that goes up.
func (b *BCTest) NodeRestart(index int) {
	b.Servers[index].Unpause()
	require.NoError(b.T, b.Services[index].TestRestart())
}

// CloseAll must be used when the test is done.
// It makes sure that the system is in an idle state before shutting it down.
func (b *BCTest) CloseAll() {
	if b.Client != nil {
		require.NoError(b.T, b.Client.WaitPropagation(-1))
	}
	b.Local.CloseAll()
}

// TxArgs can be used to define in more detail how the transactions should be
// sent to the ledger.
type TxArgs struct {
	Node            int
	Wait            int
	WaitPropagation bool
	RequireSuccess  bool
}

// TxArgsDefault represent sensible defaults for new transactions.
// They are used if the args=nil.
var TxArgsDefault = TxArgs{
	Node:            0,
	Wait:            10,
	WaitPropagation: true,
	RequireSuccess:  true,
}

// SendInst takes the instructions, adds the counters,
// and signs them using the b.Signer. If b.Signer is used
// outside of SendInst and SendTx,
// b.SignerCounter needs to be updated by the test.
// If args == nil, TxArgsDefault is used.
func (b *BCTest) SendInst(args *TxArgs,
	inst ...Instruction) (ClientTransaction, AddTxResponse) {

	for i := range inst {
		inst[i].SignerIdentities = []darc.Identity{b.Signer.Identity()}
		inst[i].SignerCounter = []uint64{b.SignerCounter}
		b.SignerCounter++
	}
	ctx := NewClientTransaction(CurrentVersion, inst...)
	h := ctx.Instructions.Hash()
	for i := range ctx.Instructions {
		require.NoError(b.T, ctx.Instructions[i].SignWith(h, b.Signer))
	}

	return ctx, b.SendTx(args, ctx)
}

// SendTx calls the service of the node given in args and adds a transaction.
// Depending on args,
// it checks for success and waits for the transaction to be stored in all
// nodes.
// If args == nil, TxArgsDefault is used.
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

// SpawnDummy creates a new dummy-instance with the value "anyvalue".
// If args == nil, TxArgsDefault is used.
func (b *BCTest) SpawnDummy(args *TxArgs) (ClientTransaction, AddTxResponse) {
	return b.SendInst(args, Instruction{
		InstanceID: NewInstanceID(b.GenesisDarc.GetBaseID()),
		Spawn: &Spawn{
			ContractID: DummyContractName,
			Args:       Arguments{{Name: "data", Value: []byte("anyvalue")}},
		},
	})
}

// DummyContractName is the name of the dummy contract.
const DummyContractName = "dummy"

// dummyContract is a copy of the value-contract
type dummyContract struct {
	BasicContract
	Data []byte
}

func dummyContractFromBytes(in []byte) (Contract, error) {
	return &dummyContract{Data: in}, nil
}

func (dc *dummyContract) Spawn(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
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

func (dc *dummyContract) Invoke(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	_, _, _, darcID, err := cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	return []StateChange{
		NewStateChange(Update, inst.InstanceID, DummyContractName, inst.Invoke.Args[0].Value, darcID),
	}, nil, nil
}

func (dc *dummyContract) Delete(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	_, _, _, darcID, err := cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	return []StateChange{
		NewStateChange(Remove, inst.InstanceID, "", nil, darcID),
	}, nil, nil
}
