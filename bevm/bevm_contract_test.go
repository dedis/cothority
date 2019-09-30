package bevm

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3/log"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
)

var txParams = struct {
	GasLimit uint64
	GasPrice *big.Int
}{uint64(1e7), big.NewInt(1)}

var testPrivateKeys = []string{
	"c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
	"ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f",
	"8503d4206b83002eee8ffe8a11c2b09885a0912f5cddd2401d96c3abccca7401",
	"f78572bd69fbd3118ab756e3544d23821a2002b137c9037a3b8fd5b09169a73c",
}

// Spawn a BEvm
func Test_Spawn(t *testing.T) {
	log.LLvl1("BEvm instantiation")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	_, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.Nil(t, err)
}

// Spawn and delete a BEvm instance
func Test_SpawnAndDelete(t *testing.T) {
	log.LLvl1("BEvm creation and deletion")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.Nil(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.Nil(t, err)

	// Initialize an account
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.Nil(t, err)

	// Credit the account
	err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.Nil(t, err)

	// Deploy a Candy contract
	candySupply := big.NewInt(100)
	candyContract, err := NewEvmContract(getContractPath(t, "Candy"))
	require.Nil(t, err)
	err = bevmClient.Deploy(txParams.GasLimit, txParams.GasPrice, 0, a, candyContract, candySupply)
	require.Nil(t, err)

	// Delete the BEvm instance
	err = bevmClient.Delete()
	require.Nil(t, err)
}

// Credit and display three accounts balances
func Test_InvokeCreditAccounts(t *testing.T) {
	log.LLvl1("Account credit and balance")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.Nil(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.Nil(t, err)

	// Initialize some accounts
	accounts := []*EvmAccount{}
	for _, privKey := range testPrivateKeys {
		account, err := NewEvmAccount(privKey)
		require.Nil(t, err)
		accounts = append(accounts, account)
	}

	// Credit each account and check its balance
	for i, account := range accounts {
		amount := big.NewInt(int64((i + 1) * WeiPerEther))

		err = bevmClient.CreditAccount(amount, account.Address)
		require.Nil(t, err)

		balance, err := bevmClient.GetAccountBalance(account.Address)
		require.Nil(t, err)

		require.Equal(t, amount, balance)
	}
}

func Test_InvokeCandyContract(t *testing.T) {
	log.LLvl1("Candy")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.Nil(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.Nil(t, err)

	// Initialize an account
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.Nil(t, err)

	// Credit the account
	err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.Nil(t, err)

	// Deploy a Candy contract
	candySupply := big.NewInt(100)
	candyContract, err := NewEvmContract(getContractPath(t, "Candy"))
	require.Nil(t, err)
	err = bevmClient.Deploy(txParams.GasLimit, txParams.GasPrice, 0, a, candyContract, candySupply)
	require.Nil(t, err)

	// Get initial candy balance
	candyBalance := big.NewInt(0)
	err = bevmClient.Call(a, &candyBalance, candyContract, "getRemainingCandies")
	require.Nil(t, err)
	require.Equal(t, candySupply, candyBalance)

	// Eat 10 candies
	err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, a, candyContract, "eatCandy", big.NewInt(10))
	require.Nil(t, err)

	// Get remaining candies
	expectedCandyBalance := big.NewInt(90)
	err = bevmClient.Call(a, &candyBalance, candyContract, "getRemainingCandies")
	require.Nil(t, err)
	require.Equal(t, expectedCandyBalance, candyBalance)
}

func Test_Time(t *testing.T) {
	log.LLvl1("TimeTest")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.Nil(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.Nil(t, err)

	// Initialize an account
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.Nil(t, err)

	// Credit the account
	err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.Nil(t, err)

	// Deploy a TimeTest contract
	contract, err := NewEvmContract(getContractPath(t, "TimeTest"))
	require.Nil(t, err)
	err = bevmClient.Deploy(txParams.GasLimit, txParams.GasPrice, 0, a, contract)
	require.Nil(t, err)

	// Get current block time
	expectedTime := big.NewInt(12345) // Currently hardcoded in getContext()
	time := big.NewInt(0)
	err = bevmClient.Call(a, &time, contract, "getTime")
	require.Nil(t, err)
	require.Equal(t, expectedTime, time)
}

func Test_InvokeTokenContract(t *testing.T) {
	log.LLvl1("ERC20Token")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.Nil(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.Nil(t, err)

	// Initialize two accounts
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.Nil(t, err)
	b, err := NewEvmAccount(testPrivateKeys[1])
	require.Nil(t, err)

	// Credit the accounts
	err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.Nil(t, err)
	err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), b.Address)
	require.Nil(t, err)

	// Deploy an ERC20 Token contract
	erc20Contract, err := NewEvmContract(getContractPath(t, "ERC20Token"))
	require.Nil(t, err)
	err = bevmClient.Deploy(txParams.GasLimit, txParams.GasPrice, 0, a, erc20Contract)
	require.Nil(t, err)

	// Retrieve the total supply
	supply := big.NewInt(0)
	err = bevmClient.Call(a, &supply, erc20Contract, "totalSupply")
	require.Nil(t, err)

	balance := big.NewInt(0)

	// A's initial balance should be the total supply, as he is the owner
	err = bevmClient.Call(a, &balance, erc20Contract, "balanceOf", a.Address)
	require.Nil(t, err)
	require.Equal(t, supply, balance)

	// B's initial balance should be empty
	err = bevmClient.Call(a, &balance, erc20Contract, "balanceOf", b.Address)
	require.Nil(t, err)
	assertBigInt0(t, balance)

	// Transfer 100 tokens from A to B
	err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, a, erc20Contract, "transfer", b.Address, big.NewInt(100))
	require.Nil(t, err)

	// Check the new balances
	newA := new(big.Int).Sub(supply, big.NewInt(100))
	newB := big.NewInt(100)

	err = bevmClient.Call(a, &balance, erc20Contract, "balanceOf", a.Address)
	require.Nil(t, err)
	require.Equal(t, newA, balance)

	err = bevmClient.Call(a, &balance, erc20Contract, "balanceOf", b.Address)
	require.Nil(t, err)
	require.Equal(t, newB, balance)

	// Try to transfer 101 tokens from B to A; this should be rejected by the EVM
	err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, b, erc20Contract, "transfer", a.Address, big.NewInt(101))
	require.Nil(t, err)

	// Check that the balances have not changed
	err = bevmClient.Call(a, &balance, erc20Contract, "balanceOf", a.Address)
	require.Nil(t, err)
	require.Equal(t, newA, balance)

	err = bevmClient.Call(a, &balance, erc20Contract, "balanceOf", b.Address)
	require.Nil(t, err)
	require.Equal(t, newB, balance)
}

func Test_InvokeLoanContract(t *testing.T) {
	log.LLvl1("LoanContract")
	//Preparing ledger
	bct := newBCTest(t)
	defer bct.Close()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.cl, bct.signer, bct.gDarc)
	require.Nil(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.cl, bct.signer, instanceID)
	require.Nil(t, err)

	// Initialize two accounts
	a, err := NewEvmAccount(testPrivateKeys[0])
	require.Nil(t, err)
	b, err := NewEvmAccount(testPrivateKeys[1])
	require.Nil(t, err)

	// Credit the accounts
	err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), a.Address)
	require.Nil(t, err)
	err = bevmClient.CreditAccount(big.NewInt(5*WeiPerEther), b.Address)
	require.Nil(t, err)

	// Deploy an ERC20 Token contract
	erc20Contract, err := NewEvmContract(getContractPath(t, "ERC20Token"))
	require.Nil(t, err)
	err = bevmClient.Deploy(txParams.GasLimit, txParams.GasPrice, 0, a, erc20Contract)
	require.Nil(t, err)

	// Deploy a Loan contract
	guarantee := big.NewInt(10000)
	loanAmount := big.NewInt(1.5 * WeiPerEther)

	loanContract, err := NewEvmContract(getContractPath(t, "LoanContract"))
	require.Nil(t, err)
	err = bevmClient.Deploy(txParams.GasLimit, txParams.GasPrice, 0, a, loanContract,
		loanAmount,            // wantedAmount: the amount in Ether that the borrower wants to borrow
		big.NewInt(0),         // interest: the amount in Ether that the borrower will pay pack in addition to the borrowed amount
		guarantee,             // tokenAmount: the number of tokens provided by the borrower as guarantee
		"TestCoin",            // tokenName: the name of the tokens used as guarantee
		erc20Contract.Address, // tokenContractAddress: the address of the ERC20 contract handling the tokens used as guarantee
		big.NewInt(0),         // length: the duration of the loan, in days
	)
	require.Nil(t, err)

	getBalances := func(from *EvmAccount, address common.Address) (tokenBalance, balance *big.Int) {
		tokenBalance = big.NewInt(0)
		err = bevmClient.Call(from, &tokenBalance, erc20Contract, "balanceOf", address)
		require.Nil(t, err)
		balance, err = bevmClient.GetAccountBalance(address)
		require.Nil(t, err)
		return
	}

	var tokBal, bal *big.Int

	// Initially, the guarantee is empty
	initTokBalA, _ := getBalances(a, a.Address)
	tokBal, _ = getBalances(a, loanContract.Address)
	assertBigInt0(t, tokBal)

	// Transfer tokens from A as a guarantee (A owns all the tokens as he deployed the Token contract)
	err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, a, erc20Contract, "transfer", loanContract.Address, guarantee)
	require.Nil(t, err)

	tokBal, _ = getBalances(a, a.Address)
	expected := new(big.Int).Sub(initTokBalA, guarantee)
	require.Equal(t, expected, tokBal)

	tokBal, _ = getBalances(a, loanContract.Address)
	require.Equal(t, guarantee, tokBal)

	// Check that there are enough tokens
	err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, 0, a, loanContract, "checkTokens")
	require.Nil(t, err)

	// Lend
	_, initEtherBalA := getBalances(a, a.Address)

	err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, loanAmount.Uint64(), b, loanContract, "lend")
	require.Nil(t, err)

	_, bal = getBalances(a, a.Address)
	expected = new(big.Int).Add(initEtherBalA, loanAmount)
	require.Equal(t, expected, bal)

	// Pay back
	_, initEtherBalB := getBalances(a, b.Address)

	err = bevmClient.Transaction(txParams.GasLimit, txParams.GasPrice, loanAmount.Uint64(), a, loanContract, "payback")
	require.Nil(t, err)

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
	out.gMsg, err = byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, out.roster,
		[]string{"spawn:bevm", "invoke:bevm.credit", "invoke:bevm.transaction", "delete:bevm"},
		out.signer.Identity())
	require.Nil(t, err)
	out.gDarc = &out.gMsg.GenesisDarc

	// This BlockInterval is good for testing, but in real world applications this
	// should be more like 5 seconds.
	out.gMsg.BlockInterval = time.Second

	out.cl, _, err = byzcoin.NewLedger(out.gMsg, false)
	require.Nil(t, err)

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

func getContractPath(t *testing.T, name string) string {
	// Test contracts are located in the "testdata" subdirectory, in a
	// subdirectory named after the contract, and in files named
	// <name>_sol_<name>.{abi,bin}

	curDir, err := os.Getwd()
	require.Nil(t, err)

	return filepath.Join(curDir, "testdata", name, name+"_sol_"+name)
}
