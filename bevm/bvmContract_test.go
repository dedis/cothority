package bevm

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
)

const WeiPerEther = 1e18

type TransactionParameters struct {
	GasLimit uint64
	GasPrice *big.Int
}

var txParams TransactionParameters = TransactionParameters{GasLimit: uint64(1e7), GasPrice: big.NewInt(1)}

var testAddresses = []string{
	"0x627306090abab3a6e1400e9345bc60c78a8bef57",
	"0xf17f52151ebef6c7334fad080c5704d77216b732",
	"0xB8C7e1fAA6Cb23690fc068D6Be3d7Ad4dC16Ba78",
	"0xf3250dbB0640b292e33d44De3D7E6C94E4D034C9",
}
var testPrivateKeys = []string{
	"c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
	"ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f",
	"8503d4206b83002eee8ffe8a11c2b09885a0912f5cddd2401d96c3abccca7401",
	"f78572bd69fbd3118ab756e3544d23821a2002b137c9037a3b8fd5b09169a73c",
}

// Helper function to handle the case of the "strange big.Int 0", as can be
// returned sometimes with the Ethereum unpacking of a result, and causes
// require.Equal() to return false:
//
// 		big.NewInt(0).Bits()	--> []big.Word(nil)
// 		value.Bits()			--> []big.Word([])
func assertBigInt0(t *testing.T, actual *big.Int) {
	require.Equal(t, 0, big.NewInt(0).Cmp(actual))
}

// Spawn a bvm
func Test_Spawn(t *testing.T) {
	log.LLvl1("test: instantiating evm")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	bct.createInstance(byzcoin.Arguments{})
}

// Credit and display an account balance
func Test_InvokeCredit(t *testing.T) {
	log.LLvl1("test: crediting and displaying an account balance")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	instID := bct.createInstance(byzcoin.Arguments{})

	// Initialize an account
	account, err := NewEvmAccount(testAddresses[0], testPrivateKeys[0])
	require.Nil(t, err)

	amount := big.NewInt(3.1415926535 * WeiPerEther)

	// Credit the account
	bct.creditAccounts(instID, amount, account.Address)

	// Retrieve its balance
	balance, err := getAccountBalance(bct.cl, instID, account.Address)
	require.Nil(t, err)

	require.Equal(t, amount, balance)
}

// Credit and display three accounts balances
func Test_InvokeCreditAccounts(t *testing.T) {
	log.LLvl1("test: crediting and checking accounts balances")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	instID := bct.createInstance(byzcoin.Arguments{})

	// Initialize some accounts
	accounts := []*EvmAccount{}
	for i, _ := range testAddresses {
		account, err := NewEvmAccount(testAddresses[i], testPrivateKeys[i])
		require.Nil(t, err)
		accounts = append(accounts, account)
	}

	// Credit each account and check its balance
	for i, account := range accounts {
		amount := big.NewInt(int64((i + 1) * WeiPerEther))

		bct.creditAccounts(instID, amount, account.Address)

		balance, err := getAccountBalance(bct.cl, instID, account.Address)
		require.Nil(t, err)

		require.Equal(t, amount, balance)
	}
}

func (bct *bcTest) creditAccounts(instID byzcoin.InstanceID, amount *big.Int, addresses ...common.Address) {
	for _, address := range addresses {
		bct.invokeInstance(instID, "credit", byzcoin.Arguments{
			{Name: "address", Value: address.Bytes()},
			{Name: "amount", Value: amount.Bytes()},
		})
	}
}

func (bct *bcTest) displayAccounts(instID byzcoin.InstanceID, addresses ...common.Address) {
	for _, address := range addresses {
		bct.invokeInstance(instID, "display", byzcoin.Arguments{
			{Name: "address", Value: address.Bytes()},
		})
	}
}

func (bct *bcTest) deploy(instID byzcoin.InstanceID, txParams TransactionParameters, value uint64, account *EvmAccount, contract *EvmContract, args ...interface{}) error {
	packedArgs, err := contract.packConstructor(args...)
	if err != nil {
		return err
	}

	callData := append(contract.Bytecode, packedArgs...)
	deployTx := types.NewContractCreation(account.Nonce, big.NewInt(int64(value)), txParams.GasLimit, txParams.GasPrice, callData)
	signedTxBuffer, err := account.signAndMarshalTx(deployTx)
	require.Nil(bct.t, err)

	bct.invokeInstance(instID, "transaction", byzcoin.Arguments{
		{Name: "tx", Value: signedTxBuffer},
	})

	//log.LLvl1("deployed new contract at", crypto.CreateAddress(common.HexToAddress(A), deployTx.Nonce()).Hex())
	//log.LLvl1("nonce tx", deployTx.Nonce(), "should check", nonce)

	contract.Address = crypto.CreateAddress(account.Address, account.Nonce)
	account.Nonce += 1

	return nil
}

func (bct *bcTest) transact(instID byzcoin.InstanceID, txParams TransactionParameters, value uint64, account *EvmAccount, contract EvmContract, method string, args ...interface{}) error {
	log.LLvl1("Invoking Byzcoin for EVM method:", method)

	callData, err := contract.packMethod(method, args...)
	if err != nil {
		return err
	}

	deployTx := types.NewTransaction(account.Nonce, contract.Address, big.NewInt(int64(value)), txParams.GasLimit, txParams.GasPrice, callData)
	signedTxBuffer, err := account.signAndMarshalTx(deployTx)
	require.Nil(bct.t, err)

	bct.invokeInstance(instID, "transaction", byzcoin.Arguments{
		{Name: "tx", Value: signedTxBuffer},
	})

	account.Nonce += 1

	return nil
}

func getEvmDb(client *byzcoin.Client, instID byzcoin.InstanceID) (*EvmDb, error) {
	// Retrieve the proof of the Byzcoin instance
	proofResponse, err := client.GetProof(instID[:])
	if err != nil {
		return nil, err
	}

	// Validate the proof
	err = proofResponse.Proof.Verify(client.ID)
	if err != nil {
		return nil, err
	}

	// Extract the value from the proof
	_, value, _, _, err := proofResponse.Proof.KeyValue()
	if err != nil {
		return nil, err
	}

	// Decode the proof value into an EVM State
	var es ES
	err = protobuf.Decode(value, &es)
	if err != nil {
		return nil, err
	}
	evmDb, err := NewEvmDb(&es)
	if err != nil {
		return nil, err
	}

	return evmDb, nil
}

func getAccountBalance(client *byzcoin.Client, instID byzcoin.InstanceID, address common.Address) (*big.Int, error) {
	evmDb, err := getEvmDb(client, instID)
	if err != nil {
		return nil, err
	}

	balance := evmDb.stateDb.GetBalance(address)

	log.Lvl1("balance of", address.Hex(), ":", balance, "wei")

	return balance, nil
}

func (bct *bcTest) call(instID byzcoin.InstanceID, account *EvmAccount, result interface{}, contract EvmContract, method string, args ...interface{}) error {
	// Pack the method call and arguments
	callData, err := contract.packMethod(method, args...)
	if err != nil {
		return err
	}

	// Retrieve the EVM state
	evmDb, err := getEvmDb(bct.cl, instID)
	if err != nil {
		return err
	}

	// Instantiate a new EVM
	evm := vm.NewEVM(getContext(), evmDb.stateDb, getChainConfig(), getVMConfig())

	// Perform the call (1 Ether should be enough for everyone [tm]...)
	ret, _, err := evm.Call(vm.AccountRef(account.Address), contract.Address, callData, uint64(1*WeiPerEther), big.NewInt(0))
	if err != nil {
		return err
	}

	// Unpack the result into the caller's variable
	err = contract.unpackResult(&result, method, ret)
	if err != nil {
		return err
	}

	return nil
}

func Test_InvokeToken(t *testing.T) {
	log.LLvl1("test: ERC20Token")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	instID := bct.createInstance(byzcoin.Arguments{})

	// Initialize two accounts
	a, err := NewEvmAccount(testAddresses[0], testPrivateKeys[0])
	require.Nil(t, err)
	b, err := NewEvmAccount(testAddresses[1], testPrivateKeys[1])
	require.Nil(t, err)

	// Credit the accounts
	bct.creditAccounts(instID, big.NewInt(5*WeiPerEther), a.Address, b.Address)

	// Deploy an ERC20 Token contract
	erc20Contract, err := getSmartContract("ERC20Token")
	require.Nil(t, err)
	err = bct.deploy(instID, txParams, 0, a, erc20Contract)
	require.Nil(t, err)

	// Rretrieve the total supply
	supply := big.NewInt(0)
	err = bct.call(instID, a, &supply, *erc20Contract, "totalSupply")
	require.Nil(t, err)

	balance := big.NewInt(0)

	// A's initial balance should be the total supply, as he is the owner
	err = bct.call(instID, a, &balance, *erc20Contract, "balanceOf", a.Address)
	require.Nil(t, err)
	require.Equal(t, supply, balance)

	// B's initial balance should be empty
	err = bct.call(instID, a, &balance, *erc20Contract, "balanceOf", b.Address)
	require.Nil(t, err)
	assertBigInt0(t, balance)

	// Transfer 100 tokens from A to B
	err = bct.transact(instID, txParams, 0, a, *erc20Contract, "transfer", b.Address, big.NewInt(100))
	require.Nil(t, err)

	// Check the new balances
	newA := new(big.Int).Sub(supply, big.NewInt(100))
	newB := big.NewInt(100)

	err = bct.call(instID, a, &balance, *erc20Contract, "balanceOf", a.Address)
	require.Nil(t, err)
	require.Equal(t, newA, balance)

	err = bct.call(instID, a, &balance, *erc20Contract, "balanceOf", b.Address)
	require.Nil(t, err)
	require.Equal(t, newB, balance)

	// Try to transfer 101 tokens from B to A; this should fail
	err = bct.transact(instID, txParams, 0, b, *erc20Contract, "transfer", a.Address, big.NewInt(101))
	require.Nil(t, err)

	// Check that the balances have not changed
	err = bct.call(instID, a, &balance, *erc20Contract, "balanceOf", a.Address)
	require.Nil(t, err)
	require.Equal(t, newA, balance)

	err = bct.call(instID, a, &balance, *erc20Contract, "balanceOf", b.Address)
	require.Nil(t, err)
	require.Equal(t, newB, balance)
}

func Test_InvokeLoanContract(t *testing.T) {
	log.LLvl1("Deploying Loan Contract")
	//Preparing ledger
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	instID := bct.createInstance(byzcoin.Arguments{})

	// Initialize two accounts
	a, err := NewEvmAccount(testAddresses[0], testPrivateKeys[0])
	require.Nil(t, err)
	b, err := NewEvmAccount(testAddresses[1], testPrivateKeys[1])
	require.Nil(t, err)

	// Credit the accounts
	bct.creditAccounts(instID, big.NewInt(5*WeiPerEther), a.Address, b.Address)

	// Deploy an ERC20 Token contract
	erc20Contract, err := getSmartContract("ERC20Token")
	require.Nil(t, err)
	err = bct.deploy(instID, txParams, 0, a, erc20Contract)
	require.Nil(t, err)

	// Deploy a Loan contract
	guarantee := big.NewInt(10000)
	loanAmount := big.NewInt(1.5 * WeiPerEther)

	loanContract, err := getSmartContract("LoanContract")
	require.Nil(t, err)
	err = bct.deploy(instID, txParams, 0, a, loanContract,
		loanAmount,            // wantedAmount: the amount in Ether that the borrower wants to borrow
		big.NewInt(0),         // interest: the amount in Ether that the borrower will pay pack in addition to the borrowed amount
		guarantee,             // tokenAmount: the number of tokens provided by the borrower as guarantee
		"TestCoin",            // tokenName: the name of the tokens used as guarantee
		erc20Contract.Address, // tokenContractAddress: the address of the ERC20 contract handling the tokens used as guarantee
		big.NewInt(0),         // length: the duration of the loan, in days
	)
	require.Nil(t, err)

	getBalances := func(account *EvmAccount, address common.Address) (tokenBalance, balance *big.Int) {
		tokenBalance = big.NewInt(0)
		err = bct.call(instID, account, &tokenBalance, *erc20Contract, "balanceOf", address)
		require.Nil(t, err)
		balance, err = getAccountBalance(bct.cl, instID, address)
		require.Nil(t, err)
		return
	}

	var tokBal, bal *big.Int

	// Initially, the guarantee is empty
	tokBal, _ = getBalances(a, loanContract.Address)
	initTokBalA, _ := getBalances(a, a.Address)
	assertBigInt0(t, tokBal)

	// Transfer tokens from A as a guarantee (A owns all the tokens as he deployed the Token contract)
	err = bct.transact(instID, txParams, 0, a, *erc20Contract, "transfer", loanContract.Address, guarantee)
	require.Nil(t, err)

	tokBal, _ = getBalances(a, a.Address)
	newTokBalA := new(big.Int).Sub(initTokBalA, guarantee)
	require.Equal(t, newTokBalA, tokBal)

	tokBal, _ = getBalances(a, loanContract.Address)
	require.Equal(t, guarantee, tokBal)

	// Check that there are enough tokens
	err = bct.transact(instID, txParams, 0, a, *loanContract, "checkTokens")
	require.Nil(t, err)

	// Lend
	_, initEtherBalA := getBalances(a, a.Address)

	err = bct.transact(instID, txParams, loanAmount.Uint64(), b, *loanContract, "lend")
	require.Nil(t, err)

	_, bal = getBalances(a, a.Address)
	newEtherBalA := new(big.Int).Add(initEtherBalA, loanAmount)
	require.Equal(t, newEtherBalA, bal)

	// Pay back
	_, initEtherBalB := getBalances(a, b.Address)

	err = bct.transact(instID, txParams, loanAmount.Uint64(), a, *loanContract, "payback")
	require.Nil(t, err)

	_, bal = getBalances(a, b.Address)
	newEtherBalB := new(big.Int).Add(initEtherBalB, loanAmount)
	require.Equal(t, newEtherBalB, bal)
}

//Signs the transaction with a private key and returns the transaction in byte format, ready to be included into the Byzcoin transaction
func (account EvmAccount) signAndMarshalTx(tx *types.Transaction) ([]byte, error) {
	var signer types.Signer = types.HomesteadSigner{}
	signedTx, err := types.SignTx(tx, signer, account.PrivateKey)
	if err != nil {
		return nil, err
	}
	signedBuffer, err := signedTx.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return signedBuffer, err
}

// bcTest is used here to provide some simple test structure for different
// tests.
type bcTest struct {
	t       *testing.T
	local   *onet.LocalTest
	signer  darc.Signer
	servers []*onet.Server
	roster  *onet.Roster
	cl      *byzcoin.Client
	gMsg    *byzcoin.CreateGenesisBlock
	gDarc   *darc.Darc
	ct      uint64
}

func newBCTest(t *testing.T) (out *bcTest) {
	out = &bcTest{t: t}
	// First create a local test environment with three nodes.
	out.local = onet.NewTCPTest(cothority.Suite)

	out.signer = darc.NewSignerEd25519(nil, nil)
	out.servers, out.roster, _ = out.local.GenTree(3, true)

	// Then create a new ledger with the genesis darc having the right
	// to create and update keyValue contracts.
	var err error
	out.gMsg, err = byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, out.roster,
		[]string{"spawn:bvm", "invoke:bvm.display", "invoke:bvm.credit", "invoke:bvm.transaction"}, out.signer.Identity())
	require.Nil(t, err)
	out.gDarc = &out.gMsg.GenesisDarc

	// This BlockInterval is good for testing, but in real world applications this
	// should be more like 5 seconds.
	out.gMsg.BlockInterval = time.Second

	out.cl, _, err = byzcoin.NewLedger(out.gMsg, false)
	require.Nil(t, err)
	out.ct = 1

	return out
}

func (bct *bcTest) Close() {
	bct.local.CloseAll()
}

//The following functions are Byzcoin transactions (instances) that will cary either the Ethereum transactions or
// a credit and display command

func (bct *bcTest) createInstance(args byzcoin.Arguments) byzcoin.InstanceID {
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID:    byzcoin.NewInstanceID(bct.gDarc.GetBaseID()),
			SignerCounter: []uint64{bct.ct},
			Spawn: &byzcoin.Spawn{
				ContractID: ContractBvmID,
				Args:       args,
			},
		}},
	}
	bct.ct++
	// And we need to sign the instruction with the signer that has his
	// public key stored in the darc.
	require.NoError(bct.t, ctx.FillSignersAndSignWith(bct.signer))

	// Sending this transaction to ByzCoin does not directly include it in the
	// global state - first we must wait for the new block to be created.
	var err error
	_, err = bct.cl.AddTransactionAndWait(ctx, 20)
	require.Nil(bct.t, err)
	return ctx.Instructions[0].DeriveID("")
}

func (bct *bcTest) invokeInstance(instID byzcoin.InstanceID, command string, args byzcoin.Arguments) {
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID:    instID,
			SignerCounter: []uint64{bct.ct},
			Invoke: &byzcoin.Invoke{
				ContractID: "bvm",
				Command:    command,
				Args:       args,
			},
		}},
	}
	bct.ct++

	// And we need to sign the instruction with the signer that has his
	// public key stored in the darc.
	require.NoError(bct.t, ctx.FillSignersAndSignWith(bct.signer))

	// Sending this transaction to ByzCoin does not directly include it in the
	// global state - first we must wait for the new block to be created.
	var err error
	_, err = bct.cl.AddTransactionAndWait(ctx, 30)
	require.Nil(bct.t, err)
}
