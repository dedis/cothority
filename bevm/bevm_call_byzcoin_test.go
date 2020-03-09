package bevm

import (
	"math/big"
	"testing"
	"time"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func init() {
	err := byzcoin.RegisterGlobalContract(myValueContractID,
		myValueContractFromBytes)
	log.ErrFatal(err)

	evmSpawnableContracts[myValueContractID] = true
}

const myValueContractID = "MyValueContract"

func myValueContractFromBytes(in []byte) (byzcoin.Contract, error) {
	return myValueContract{value: in}, nil
}

// The test value contract just holds a value
type myValueContract struct {
	byzcoin.BasicContract
	value []byte
}

func (c myValueContract) Spawn(rst byzcoin.ReadOnlyStateTrie,
	inst byzcoin.Instruction, cin []byzcoin.Coin) (sc []byzcoin.StateChange,
	cout []byzcoin.Coin, err error) {
	cout = cin

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to get darcID: %v", err)
	}

	var newInstanceID byzcoin.InstanceID
	seed := inst.Spawn.Args.Search("seed")
	if seed != nil {
		newInstanceID = byzcoin.ComputeNewInstanceID(myValueContractID, seed)
	} else {
		newInstanceID = byzcoin.NewInstanceID(inst.DeriveID("").Slice())
	}

	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(
			byzcoin.Create,
			newInstanceID,
			myValueContractID,
			inst.Spawn.Args.Search("value"),
			darcID),
	}

	return
}

func (c myValueContract) Invoke(rst byzcoin.ReadOnlyStateTrie,
	inst byzcoin.Instruction, cin []byzcoin.Coin) (sc []byzcoin.StateChange,
	cout []byzcoin.Coin, err error) {
	cout = cin

	// Find the darcID for this instance.
	var darcID darc.ID

	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to get darcID: %v", err)
	}

	switch inst.Invoke.Command {
	case "update":
		sc = []byzcoin.StateChange{
			byzcoin.NewStateChange(
				byzcoin.Update,
				inst.InstanceID,
				myValueContractID,
				inst.Invoke.Args.Search("value"),
				darcID),
		}
		return

	default:
		return nil, nil, xerrors.New("Value contract can only update")
	}
}

func (c myValueContract) Delete(rst byzcoin.ReadOnlyStateTrie,
	inst byzcoin.Instruction, cin []byzcoin.Coin) (sc []byzcoin.StateChange,
	cout []byzcoin.Coin, err error) {
	cout = cin

	// Find the darcID for this instance.
	var darcID darc.ID

	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to get darcID: %v", err)
	}

	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(
			byzcoin.Remove,
			inst.InstanceID,
			myValueContractID,
			nil,
			darcID),
	}
	return
}

// Inspired by byzcoin.Client.WaitProof()
func waitProofGone(t *testing.T, cl *byzcoin.Client, interval time.Duration,
	id byzcoin.InstanceID) error {
	for i := 0; i < 10; i++ {
		resp, err := cl.GetProof(id[:])
		require.NoError(t, err)
		ok, err := resp.Proof.InclusionProof.Exists(id[:])
		require.NoError(t, err)
		if !ok {
			return nil
		}
		time.Sleep(interval / 10)
	}

	return xerrors.Errorf("timeout reached and inclusion proof for %v "+
		"still exists", id)
}

func Test_BEvmCallsByzcoin(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	// Initialize DARC with rights for BEvm
	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{
			"spawn:" + ContractBEvmID,
			"invoke:" + ContractBEvmID + ".credit",
			"invoke:" + ContractBEvmID + ".transaction",
		}, signer.Identity(),
	)
	require.NoError(t, err)

	gDarc := &genesisMsg.GenesisDarc
	darcID := byzcoin.NewInstanceID(gDarc.GetBaseID())
	genesisMsg.BlockInterval = time.Second

	// Create new ledger
	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.NoError(t, err)

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(cl, signer, gDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(cl, signer, instanceID)
	require.NoError(t, err)

	// Initialize an account
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.NoError(t, err)

	// Credit the account
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.NoError(t, err)

	// Deploy a CallByzcoin contract
	callBcContract, err := NewEvmContract("CallByzcoin",
		getContractData(t, "CallByzcoin", "abi"),
		getContractData(t, "CallByzcoin", "bin"))
	require.NoError(t, err)
	_, callBcInstance, err := bevmClient.Deploy(txParams.GasLimit,
		txParams.GasPrice, 0, a, callBcContract)
	require.NoError(t, err)

	// Values used for tests
	initValue := []byte{42}
	updateValue := []byte{187}

	// Spawn a value -- fails because the DARC rule is missing
	_, err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, a,
		callBcInstance, "spawnValue",
		darcID, myValueContractID, initValue[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "refused")

	// Add rules to the DARC guarding "spawn", "invoke.update" and "delete" on
	// the value contract with the address of the deployed CallByzcoin contract
	newDarc := gDarc.Copy()
	newDarc.EvolveFrom(gDarc)

	darcExpr := darc.Identity{
		EvmContract: &darc.IdentityEvmContract{
			Address: callBcInstance.Address,
		},
	}.String()

	darcAction := "spawn:" + myValueContractID
	require.NoError(t,
		newDarc.Rules.AddRule(darc.Action(darcAction), []byte(darcExpr)))

	darcAction = "invoke:" + myValueContractID + ".update"
	require.NoError(t,
		newDarc.Rules.AddRule(darc.Action(darcAction), []byte(darcExpr)))

	darcAction = "delete:" + myValueContractID
	require.NoError(t,
		newDarc.Rules.AddRule(darc.Action(darcAction), []byte(darcExpr)))

	newDarcBuf, err := newDarc.ToProto()
	require.NoError(t, err)

	// Evolve the DARC with the new rules
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: darcID,
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

	// Spawn a value
	tx, err := bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, a,
		callBcInstance, "spawnValue",
		darcID, myValueContractID, initValue[0])
	require.NoError(t, err)

	// ID generated by the Solidity event
	seed := byzcoin.ComputeSeed(tx.Instructions[0], 0)
	valID := byzcoin.ComputeNewInstanceID(myValueContractID, seed)

	// Check that the new instance exists and holds the correct value
	_, err = cl.WaitProof(valID, 10*time.Second, initValue)
	require.NoError(t, err)

	// Update the value
	_, err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, a,
		callBcInstance, "updateValue",
		valID, myValueContractID, updateValue[0])
	require.NoError(t, err)

	// Check it holds the updated value
	_, err = cl.WaitProof(valID, 10*time.Second, updateValue)
	require.NoError(t, err)

	// Delete the value instance
	_, err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, a,
		callBcInstance, "deleteValue",
		valID, myValueContractID)
	require.NoError(t, err)

	// Check it no longer exists
	require.NoError(t, waitProofGone(t, cl, 10*time.Second, valID))
}

func Test_DirectlyUseEvmIdentity(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.Signer{
		EvmContract: &darc.SignerEvmContract{
			Address: common.HexToAddress(
				"000102030405060708090A0B0C0D0E0F10111213"),
		},
	}
	_, roster, _ := local.GenTree(3, true)

	// Initialize DARC with rights to spawn a value contract using an EVM
	// contract address
	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{
			"spawn:" + myValueContractID,
		}, signer.Identity(),
	)
	require.NoError(t, err)

	gDarc := &genesisMsg.GenesisDarc
	genesisMsg.BlockInterval = time.Second

	// Create new ledger
	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.NoError(t, err)

	// Spawn a new value contract
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: myValueContractID,
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: []byte{42},
			}, {
				Name:  "id",
				Value: []byte{1, 2, 3},
			}},
		},
		SignerCounter: []uint64{getNextCounter(t, cl, signer)},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	// fails because directly using an EVM contract as signer is forbidden
	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)
	require.Contains(t, err.Error(), "forbidden signer identity")
}

func Test_SpawnTwoValues(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	// Initialize DARC with rights for BEvm
	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{
			"spawn:" + ContractBEvmID,
			"invoke:" + ContractBEvmID + ".credit",
			"invoke:" + ContractBEvmID + ".transaction",
		}, signer.Identity(),
	)
	require.NoError(t, err)

	gDarc := &genesisMsg.GenesisDarc
	darcID := byzcoin.NewInstanceID(gDarc.GetBaseID())
	genesisMsg.BlockInterval = time.Second

	// Create new ledger
	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.NoError(t, err)

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(cl, signer, gDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(cl, signer, instanceID)
	require.NoError(t, err)

	// Initialize an account
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.NoError(t, err)

	// Credit the account
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.NoError(t, err)

	// Deploy a CallByzcoin contract
	callBcContract, err := NewEvmContract("CallByzcoin",
		getContractData(t, "CallByzcoin", "abi"),
		getContractData(t, "CallByzcoin", "bin"))
	require.NoError(t, err)
	_, callBcInstance, err := bevmClient.Deploy(txParams.GasLimit,
		txParams.GasPrice, 0, a, callBcContract)
	require.NoError(t, err)

	// Add rules to the DARC guarding "spawn" on the value contract with the
	// address of the deployed CallByzcoin contract
	newDarc := gDarc.Copy()
	newDarc.EvolveFrom(gDarc)

	darcExpr := darc.Identity{
		EvmContract: &darc.IdentityEvmContract{
			Address: callBcInstance.Address,
		},
	}.String()

	darcAction := "spawn:" + myValueContractID
	require.NoError(t,
		newDarc.Rules.AddRule(darc.Action(darcAction), []byte(darcExpr)))

	// Evolve the DARC with the new rules
	newDarcBuf, err := newDarc.ToProto()
	require.NoError(t, err)

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: darcID,
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

	// Spawn a value
	initValue := []byte{42}

	tx, err := bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, a,
		callBcInstance, "spawnTwoValues",
		darcID, myValueContractID, initValue[0])
	require.NoError(t, err)

	// IDs generated by the Solidity event
	seed := byzcoin.ComputeSeed(tx.Instructions[0], 0)
	valID1 := byzcoin.ComputeNewInstanceID(myValueContractID, seed)

	seed = byzcoin.ComputeSeed(tx.Instructions[0], 1)
	valID2 := byzcoin.ComputeNewInstanceID(myValueContractID, seed)

	// Check that the new instances exist and hold the correct value
	_, err = cl.WaitProof(valID1, 10*time.Second, initValue)
	require.NoError(t, err)
	_, err = cl.WaitProof(valID2, 10*time.Second, initValue)
	require.NoError(t, err)
}

func Test_SpawnWhitelist(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	// Initialize DARC with rights for BEvm
	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{
			"spawn:" + ContractBEvmID,
			"invoke:" + ContractBEvmID + ".credit",
			"invoke:" + ContractBEvmID + ".transaction",
		}, signer.Identity(),
	)
	require.NoError(t, err)

	gDarc := &genesisMsg.GenesisDarc
	darcID := byzcoin.NewInstanceID(gDarc.GetBaseID())
	genesisMsg.BlockInterval = time.Second

	// Create new ledger
	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.NoError(t, err)

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(cl, signer, gDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(cl, signer, instanceID)
	require.NoError(t, err)

	// Initialize an account
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.NoError(t, err)

	// Credit the account
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.NoError(t, err)

	// Deploy a CallByzcoin contract
	callBcContract, err := NewEvmContract("CallByzcoin",
		getContractData(t, "CallByzcoin", "abi"),
		getContractData(t, "CallByzcoin", "bin"))
	require.NoError(t, err)
	_, callBcInstance, err := bevmClient.Deploy(txParams.GasLimit,
		txParams.GasPrice, 0, a, callBcContract)
	require.NoError(t, err)

	// Spawning a non-whitelisted contract fails
	_, err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, a,
		callBcInstance, "spawnValue",
		darcID, "xyzzy", uint8(42))
	require.Error(t, err)
	require.Contains(t, err.Error(), "not been whitelisted")
}
