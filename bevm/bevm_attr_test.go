package bevm

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
)

func init() {
	err := byzcoin.RegisterGlobalContract(valueContractID,
		valueContractFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
}

const valueContractID = "TestValueContract"

func valueContractFromBytes(in []byte) (byzcoin.Contract, error) {
	return valueContract{value: in}, nil
}

// The test value contracts just holds a value
type valueContract struct {
	byzcoin.BasicContract
	value []byte
}

func (c valueContract) Spawn(rst byzcoin.ReadOnlyStateTrie,
	inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange,
	cout []byzcoin.Coin, err error) {
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""),
			valueContractID, inst.Spawn.Args.Search("value"), darcID),
	}
	return
}

func (c valueContract) Invoke(rst byzcoin.ReadOnlyStateTrie,
	inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange,
	cout []byzcoin.Coin, err error) {
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID

	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.Invoke.Command {
	case "update":
		sc = []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
				valueContractID, inst.Invoke.Args.Search("value"), darcID),
		}
		return
	default:
		return nil, nil, xerrors.New("Value contract can only update")
	}
}

func (c valueContract) VerifyInstruction(rst byzcoin.ReadOnlyStateTrie,
	inst byzcoin.Instruction, ctxHash []byte) error {

	// Retrieve inherited attribute interpreters
	evalAttr := c.MakeAttrInterpreters(rst, inst)

	// Pass the contract value as "extra"
	evalAttr[BEvmAttrID] = MakeBevmAttr(rst, inst, c.value)

	return inst.VerifyWithOption(rst, ctxHash,
		&byzcoin.VerificationOptions{EvalAttr: evalAttr})
}

func getNextCounter(t *testing.T, cl *byzcoin.Client,
	signer darc.Signer) uint64 {
	counters, err := cl.GetSignerCounters(signer.Identity().String())
	require.NoError(t, err)

	return counters.Counters[0] + 1
}

func TestAttrBevm(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	// Initialize DARC with rights for BEvm and spawning a value contract
	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{
			"spawn:" + ContractBEvmID,
			"invoke:" + ContractBEvmID + ".credit",
			"invoke:" + ContractBEvmID + ".transaction",
			"spawn:" + valueContractID,
		}, signer.Identity(),
	)
	require.NoError(t, err)

	gDarc := &genesisMsg.GenesisDarc
	genesisMsg.BlockInterval = time.Second

	// Create new ledger
	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.NoError(t, err)

	// Spawn a new BEvm instance
	bevmID, err := NewBEvm(cl, signer, gDarc)
	require.NoError(t, err)
	log.LLvlf2("bevmID = %v", bevmID)

	// Create a new BEvm client
	bevmClient, err := NewClient(cl, signer, bevmID)
	require.NoError(t, err)
	require.NotNil(t, bevmClient)

	// Initialize an EVM account
	acct, err := NewEvmAccount(testPrivateKeys[0])
	require.NoError(t, err)

	// Credit the account
	err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), acct.Address)
	require.NoError(t, err)

	// Deploy a Verify contract (see Verify.sol in `testdata/Verify`)
	verifyContract, err := NewEvmContract(
		"Verify", getContractData(t, "Verify", "abi"),
		getContractData(t, "Verify", "bin"))
	require.NoError(t, err)
	verifyInstance, err := bevmClient.Deploy(txParams.GasLimit,
		txParams.GasPrice, 0, acct, verifyContract)
	require.NoError(t, err)

	// Add rule to the DARC guarding "update" on the value contract with the
	// `Verify.isGreater()` ethereum method defined in
	// bevm/testdata/Verify/Verify.sol
	newDarc := gDarc.Copy()
	newDarc.EvolveFrom(gDarc)
	darcAction := "invoke:" + valueContractID + ".update"
	darcExpr := fmt.Sprintf("%s & attr:%s:%s:%s:isGreater",
		signer.Identity().String(),
		BEvmAttrID,
		bevmID.String(),
		verifyInstance.Address.Hex())
	log.LLvlf2("DARC rule: %s → %s", darcAction, darcExpr)
	require.NoError(t,
		newDarc.Rules.AddRule(darc.Action(darcAction), []byte(darcExpr)))

	newDarcBuf, err := newDarc.ToProto()
	require.NoError(t, err)

	// Evolve the DARC with the new rule
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDarcID,
			Command:    "evolve",
			Args: byzcoin.Arguments{{
				Name:  "darc",
				Value: newDarcBuf,
			}},
		},
		SignerCounter: []uint64{getNextCounter(t, cl, signer)},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)

	// --- Beginning of actual test ;-)

	// Spawn a new value contract with state 42
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: valueContractID,
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: []byte{42},
			}},
		},
		SignerCounter: []uint64{getNextCounter(t, cl, signer)},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)

	myID := ctx.Instructions[0].DeriveID("")

	// Update fails: 41 is not > 42
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: valueContractID,
			Command:    "update",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: []byte{41},
			}},
		},
		SignerCounter: []uint64{getNextCounter(t, cl, signer)},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	resp, err := cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)
	require.Contains(t, resp.Error, "value is not greater")

	// Update succeeds: 43 > 42
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: valueContractID,
			Command:    "update",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: []byte{43},
			}},
		},
		SignerCounter: []uint64{getNextCounter(t, cl, signer)},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)

	// Invoke fails: 43 is not > 43
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: valueContractID,
			Command:    "update",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: []byte{43},
			}},
		},
		SignerCounter: []uint64{getNextCounter(t, cl, signer)},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	resp, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)
	require.Contains(t, resp.Error, "value is not greater")
}
