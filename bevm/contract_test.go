package bevm

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.dedis.ch/cothority/v3"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
)

var txParams = struct {
	GasLimit uint64
	GasPrice uint64
}{1e7, 1}

var testPrivateKeys = []string{
	"c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
	"ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f",
	"8503d4206b83002eee8ffe8a11c2b09885a0912f5cddd2401d96c3abccca7401",
	"f78572bd69fbd3118ab756e3544d23821a2002b137c9037a3b8fd5b09169a73c",
}

// Spawn a BEvm
func Test_Spawn(t *testing.T) {
	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	_, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.NoError(t, err)
}

// Spawn and delete a BEvm instance
func Test_SpawnAndDelete(t *testing.T) {
	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.NoError(t, err)

	// Initialize an account
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.NoError(t, err)

	// Credit the account
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.NoError(t, err)

	// Deploy a Candy contract
	candySupply := big.NewInt(100)
	candyContract, err := NewEvmContract(
		"Candy",
		getContractData(t, "Candy", "abi"),
		getContractData(t, "Candy", "bin"))
	require.NoError(t, err)
	_, _, err = bevmClient.Deploy(
		txParams.GasLimit, txParams.GasPrice, 0, a, candyContract,
		candySupply)
	require.NoError(t, err)

	// Delete the BEvm instance
	err = bevmClient.Delete()
	require.NoError(t, err)
}

// Credit and display three accounts balances
func Test_InvokeCreditAccounts(t *testing.T) {
	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.NoError(t, err)

	// Initialize some accounts
	accounts := []*EvmAccount{}
	for _, privKey := range testPrivateKeys {
		account, err := NewEvmAccount(privKey)
		require.NoError(t, err)
		accounts = append(accounts, account)
	}

	// Credit each account and check its balance
	for i, account := range accounts {
		amount := big.NewInt(int64((i + 1) * WeiPerEther))

		_, err = bevmClient.CreditAccount(amount, account.Address)
		require.NoError(t, err)

		balance, err := bevmClient.GetAccountBalance(account.Address)
		require.NoError(t, err)

		require.Equal(t, amount, balance)
	}
}

func Test_InvokeCandyContract(t *testing.T) {
	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.NoError(t, err)

	// Initialize an account
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.NoError(t, err)

	// Credit the account
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.NoError(t, err)

	// Deploy a Candy contract
	candySupply := big.NewInt(100)
	candyContract, err := NewEvmContract(
		"Candy",
		getContractData(t, "Candy", "abi"),
		getContractData(t, "Candy", "bin"))
	require.NoError(t, err)
	_, candyInstance, err := bevmClient.Deploy(
		txParams.GasLimit, txParams.GasPrice, 0, a, candyContract,
		candySupply)
	require.NoError(t, err)

	// Get initial candy balance
	result, err := bevmClient.Call(
		a, candyInstance, "getRemainingCandies")
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	require.Equal(t, candySupply, result[0])

	// Eat 10 candies
	_, err = bevmClient.Transaction(
		txParams.GasLimit, txParams.GasPrice, 0, a,
		candyInstance,
		"eatCandy", big.NewInt(10))
	require.NoError(t, err)

	// Get remaining candies
	expectedCandyBalance := big.NewInt(90)
	result, err = bevmClient.Call(
		a, candyInstance, "getRemainingCandies")
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	require.Equal(t, expectedCandyBalance, result[0])
}

func Test_Time(t *testing.T) {
	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.NoError(t, err)

	// Initialize an account
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.NoError(t, err)

	// Credit the account
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.NoError(t, err)

	// Deploy a TimeTest contract
	contract, err := NewEvmContract(
		"TimeTest",
		getContractData(t, "TimeTest", "abi"),
		getContractData(t, "TimeTest", "bin"))
	require.NoError(t, err)
	_, timeTestInstance, err := bevmClient.Deploy(
		txParams.GasLimit, txParams.GasPrice, 0, a, contract)
	require.NoError(t, err)

	// Get current timestamp in [s]
	now := time.Now().Unix()

	// Retrieve stored time
	result, err := bevmClient.Call(
		a, timeTestInstance, "getStoredTime")
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	storedTime := result[0]
	// Unitialized -- 0
	assertBigInt0(t, storedTime.(*big.Int))

	// Store current time
	_, err = bevmClient.Transaction(
		txParams.GasLimit, txParams.GasPrice, 0, a,
		timeTestInstance, "storeCurrentTime")
	require.NoError(t, err)

	// Retrieve stored time
	result, err = bevmClient.Call(
		a, timeTestInstance, "getStoredTime")
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	storedTime = result[0]
	// Should be within 5 sec of `now`
	require.GreaterOrEqual(t, now+5, storedTime.(*big.Int).Int64())
	require.GreaterOrEqual(t, storedTime.(*big.Int).Int64(), now)

	// Retrieve current time
	result, err = bevmClient.Call(
		a, timeTestInstance, "getCurrentTime")
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	currentTime := result[0]
	// Should be within 5 sec of `now`
	require.GreaterOrEqual(t, now+5, currentTime.(*big.Int).Int64())
	require.GreaterOrEqual(t, currentTime.(*big.Int).Int64(), now)
}

// Check that ABIv2 is supported
func Test_ABIv2(t *testing.T) {
	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.NoError(t, err)

	// Initialize an account
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.NoError(t, err)

	// Credit the account
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.NoError(t, err)

	// Deploy an ABIv2 contract
	contract, err := NewEvmContract(
		"ABIv2",
		getContractData(t, "ABIv2", "abi"),
		getContractData(t, "ABIv2", "bin"))
	require.NoError(t, err)
	_, contractInstance, err := bevmClient.Deploy(
		txParams.GasLimit, txParams.GasPrice, 0, a, contract)
	require.NoError(t, err)

	// Retrieve first n squares
	n := 10
	result, err := bevmClient.Call(
		a, contractInstance, "squares", big.NewInt(int64(n)))
	require.NoError(t, err)

	// There is a single result...
	require.Equal(t, len(result), 1)
	// ...of the expected structure...
	squares, ok := result[0].([]struct {
		V1 *big.Int
		V2 *big.Int
	})
	require.True(t, ok, "received unexpected structure")
	// ...of the expected size...
	require.Len(t, squares, n)

	/// ...with the expected content
	for _, s := range squares {
		expectedSquare := big.NewInt(0).Mul(s.V1, s.V1)
		require.Zero(t, s.V2.Cmp(expectedSquare))
	}
}

func Test_InvokeTokenContract(t *testing.T) {
	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.NoError(t, err)

	// Initialize two accounts
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.NoError(t, err)
	b, err := NewEvmAccount(testPrivateKeys[1])
	require.NoError(t, err)

	// Credit the accounts
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.NoError(t, err)
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), b.Address)
	require.NoError(t, err)

	// Deploy an ERC20 Token contract
	erc20Contract, err := NewEvmContract(
		"ERC20Token",
		getContractData(t, "ERC20Token", "abi"),
		getContractData(t, "ERC20Token", "bin"))
	require.NoError(t, err)
	_, erc20Instance, err := bevmClient.Deploy(
		txParams.GasLimit, txParams.GasPrice, 0, a, erc20Contract)
	require.NoError(t, err)

	// Retrieve the total supply
	result, err := bevmClient.Call(
		a, erc20Instance, "totalSupply")
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	supply := result[0]

	// A's initial balance should be the total supply, as he is the owner
	result, err = bevmClient.Call(
		a, erc20Instance, "balanceOf", a.Address)
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	require.Equal(t, supply, result[0])

	// B's initial balance should be empty
	result, err = bevmClient.Call(
		a, erc20Instance, "balanceOf", b.Address)
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	assertBigInt0(t, result[0].(*big.Int))

	// Transfer 100 tokens from A to B
	_, err = bevmClient.Transaction(
		txParams.GasLimit, txParams.GasPrice, 0, a,
		erc20Instance, "transfer", b.Address, big.NewInt(100))
	require.NoError(t, err)

	// Check the new balances
	newA := new(big.Int).Sub(supply.(*big.Int), big.NewInt(100))
	newB := big.NewInt(100)

	result, err = bevmClient.Call(
		a, erc20Instance, "balanceOf", a.Address)
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	require.Equal(t, newA, result[0])

	result, err = bevmClient.Call(
		a, erc20Instance, "balanceOf", b.Address)
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	require.Equal(t, newB, result[0])

	// Try to transfer 101 tokens from B to A; this should be rejected by the EVM
	_, err = bevmClient.Transaction(
		txParams.GasLimit, txParams.GasPrice, 0, b,
		erc20Instance, "transfer", a.Address, big.NewInt(101))
	require.NoError(t, err)

	// Check that the balances have not changed
	result, err = bevmClient.Call(
		a, erc20Instance, "balanceOf", a.Address)
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	require.Equal(t, newA, result[0])

	result, err = bevmClient.Call(
		a, erc20Instance, "balanceOf", b.Address)
	require.NoError(t, err)
	require.Equal(t, len(result), 1)
	require.Equal(t, newB, result[0])
}

func Test_InvokeLoanContract(t *testing.T) {
	//Preparing ledger
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.NoError(t, err)

	// Initialize two accounts
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.NoError(t, err)
	b, err := NewEvmAccount(testPrivateKeys[1])
	require.NoError(t, err)

	// Credit the accounts
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.NoError(t, err)
	_, err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), b.Address)
	require.NoError(t, err)

	// Deploy an ERC20 Token contract
	erc20Contract, err := NewEvmContract(
		"ERC20Token",
		getContractData(t, "ERC20Token", "abi"),
		getContractData(t, "ERC20Token", "bin"))
	require.NoError(t, err)
	_, erc20Instance, err := bevmClient.Deploy(
		txParams.GasLimit, txParams.GasPrice, 0, a, erc20Contract)
	require.NoError(t, err)

	// Deploy a Loan contract
	guarantee := big.NewInt(10000)
	loanAmount := big.NewInt(1.5 * WeiPerEther)

	loanContract, err := NewEvmContract(
		"LoanContract",
		getContractData(t, "LoanContract", "abi"),
		getContractData(t, "LoanContract", "bin"))
	require.NoError(t, err)
	_, loanInstance, err := bevmClient.Deploy(
		txParams.GasLimit, txParams.GasPrice, 0, a, loanContract,
		// wantedAmount: the amount in Ether that the borrower wants to borrow
		loanAmount,
		// interest: the amount in Ether that the borrower will pay pack in
		// addition to the borrowed amount
		big.NewInt(0),
		// tokenAmount: the number of tokens provided by the borrower as
		// guarantee
		guarantee,
		// tokenName: the name of the tokens used as guarantee
		"TestCoin",
		// tokenContractAddress: the address of the ERC20 contract handling the
		// tokens used as guarantee
		erc20Instance.Address,
		// length: the duration of the loan, in days
		big.NewInt(0),
	)
	require.NoError(t, err)

	getBalances := func(from *EvmAccount, address common.Address) (
		tokenBalance, balance *big.Int) {
		result, err := bevmClient.Call(
			from, erc20Instance, "balanceOf", address)
		require.NoError(t, err)
		require.Equal(t, len(result), 1)
		tokenBalance = result[0].(*big.Int)
		balance, err = bevmClient.GetAccountBalance(address)
		require.NoError(t, err)
		return
	}

	var tokBal, bal *big.Int

	// Initially, the guarantee is empty
	initTokBalA, _ := getBalances(a, a.Address)
	tokBal, _ = getBalances(a, loanInstance.Address)
	assertBigInt0(t, tokBal)

	// Transfer tokens from A as a guarantee (A owns all the tokens as he
	// deployed the Token contract)
	_, err = bevmClient.Transaction(
		txParams.GasLimit, txParams.GasPrice, 0, a,
		erc20Instance, "transfer", loanInstance.Address, guarantee)
	require.NoError(t, err)

	tokBal, _ = getBalances(a, a.Address)
	expected := new(big.Int).Sub(initTokBalA, guarantee)
	require.Equal(t, expected, tokBal)

	tokBal, _ = getBalances(a, loanInstance.Address)
	require.Equal(t, guarantee, tokBal)

	// Check that there are enough tokens
	_, err = bevmClient.Transaction(
		txParams.GasLimit, txParams.GasPrice, 0, a,
		loanInstance, "checkTokens")
	require.NoError(t, err)

	// Lend
	_, initEtherBalA := getBalances(a, a.Address)

	_, err = bevmClient.Transaction(
		txParams.GasLimit, txParams.GasPrice, loanAmount.Uint64(), b,
		loanInstance, "lend")
	require.NoError(t, err)

	_, bal = getBalances(a, a.Address)
	expected = new(big.Int).Add(initEtherBalA, loanAmount)
	require.Equal(t, expected, bal)

	// Pay back
	_, initEtherBalB := getBalances(a, b.Address)

	_, err = bevmClient.Transaction(
		txParams.GasLimit, txParams.GasPrice, loanAmount.Uint64(), a,
		loanInstance, "payback")
	require.NoError(t, err)

	_, bal = getBalances(a, b.Address)
	expected = new(big.Int).Add(initEtherBalB, loanAmount)
	require.Equal(t, expected, bal)
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
	out.gMsg, err = byzcoin.DefaultGenesisMsg(
		byzcoin.CurrentVersion,
		out.roster,
		[]string{
			"spawn:bevm",
			"invoke:bevm.credit",
			"invoke:bevm.transaction",
			"delete:bevm"},
		out.signer.Identity())
	require.NoError(t, err)
	out.gDarc = &out.gMsg.GenesisDarc

	// This BlockInterval is good for testing, but in real world applications
	// this should be more like 5 seconds.
	out.gMsg.BlockInterval = time.Second

	out.cl, _, err = byzcoin.NewLedger(out.gMsg, false)
	require.NoError(t, err)

	out.cl.UseNode(0)

	return out
}

func (bct *bcTest) Close() {
	bct.local.CloseAll()
}

// Helper functions

// Sometimes, the result of a call to an Ethereum method is unpacked to a
// big.Int value of zero which, while correct, confuses require.Equal() when
// comparing to big.NewInt(0) (it returns false).
// This seems to be due to a different internal representation:
//
// 		big.NewInt(0).Bits()		--> []big.Word(nil)
// 		value_from_Ethereum.Bits()	--> []big.Word([])
//
// This helper function handles this.
func assertBigInt0(t *testing.T, actual *big.Int) {
	require.Equal(t, 0, big.NewInt(0).Cmp(actual))
}

func getContractData(t *testing.T, name string, extension string) string {
	// Test contracts are located in the "testdata" subdirectory, in a
	// subdirectory named after the contract, and in files named
	// <name>_sol_<name>.{abi,bin}

	curDir, err := os.Getwd()
	require.NoError(t, err)

	path := filepath.Join(curDir, "testdata", name,
		fmt.Sprintf("%s_sol_%s.%s", name, name, extension))

	data, err := ioutil.ReadFile(path)
	require.NoError(t, err)

	return string(data)
}
