package bevm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3/log"
	"math/big"
	"testing"
	"time"

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

//Spawn a bvm
func Test_Spawn(t *testing.T) {
	log.LLvl1("test: instantiating evm")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	bct.createInstance(byzcoin.Arguments{})
}

//Credits and displays an account balance
func Test_InvokeCredit(t *testing.T) {
	log.LLvl1("test: crediting and displaying an account balance")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	instID := bct.createInstance(byzcoin.Arguments{})

	// Credit an account
	address := []byte("0x2afd357E96a3aCbcd01615681C1D7e3398d5fb61")
	amount := new(big.Int).SetUint64(3.1415926535 * WeiPerEther).Bytes()
	bct.creditAccountInstance(instID, byzcoin.Arguments{
		{Name: "address", Value: address},
		{Name: "amount", Value: amount},
	})

	// Display its balance
	bct.displayAccountInstance(instID, byzcoin.Arguments{
		{Name: "address", Value: address},
	})
}

//Credits and displays three accounts balances
func Test_InvokeCreditAccounts(t *testing.T) {
	log.LLvl1("test: crediting and checking accounts balances")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	instID := bct.createInstance(byzcoin.Arguments{})

	addresses := [3]string{
		"0x627306090abab3a6e1400e9345bc60c78a8bef57",
		"0xf17f52151ebef6c7334fad080c5704d77216b732",
		"0xc5fdf4076b8f3a5357c5e395ab970b5b54098fef",
	}
	for i, addr := range addresses {
		address := []byte(addr)
		amount := new(big.Int).SetUint64(uint64((i + 1) * WeiPerEther)).Bytes()

		bct.creditAccountInstance(instID, byzcoin.Arguments{
			{Name: "address", Value: address},
			{Name: "amount", Value: amount},
		})

		bct.displayAccountInstance(instID, byzcoin.Arguments{
			{Name: "address", Value: address},
		})
	}
}

func (bct *bcTest) bank(instID byzcoin.InstanceID, command string, addresses ...common.Address) {
	amount := new(big.Int).SetUint64(5 * WeiPerEther)

	for _, address := range addresses {
		switch command {
		case "credit":
			//Send credit instructions to Byzcoin and incrementing nonce counter
			bct.creditAccountInstance(instID, byzcoin.Arguments{
				{Name: "address", Value: address.Bytes()},
				{Name: "amount", Value: amount.Bytes()},
			})
			log.LLvl1("credited", address.Hex(), amount, "wei")
		case "display":
			bct.displayAccountInstance(instID, byzcoin.Arguments{
				{Name: "address", Value: address.Bytes()},
			})
		default:
			log.LLvl1("incorrect instruction")
		}
	}
}

func (bct *bcTest) deploy(instID byzcoin.InstanceID, txParams TransactionParameters, value uint64, contract *EvmContract, constructorArgs []byte, account *EvmAccount) {
	data := append(contract.Bytecode, constructorArgs...)
	deployTx := types.NewContractCreation(account.Nonce, big.NewInt(int64(value)), txParams.GasLimit, txParams.GasPrice, data)
	signedTxBuffer, err := account.signAndMarshalTx(deployTx)
	require.Nil(bct.t, err)

	bct.transactionInstance(instID, byzcoin.Arguments{
		{Name: "tx", Value: signedTxBuffer},
	})

	//log.LLvl1("deployed new contract at", crypto.CreateAddress(common.HexToAddress(A), deployTx.Nonce()).Hex())
	//log.LLvl1("nonce tx", deployTx.Nonce(), "should check", nonce)

	contract.Address = crypto.CreateAddress(account.Address, account.Nonce)
	account.Nonce += 1
}

func (bct *bcTest) transact(instID byzcoin.InstanceID, txParams TransactionParameters, value uint64, contract EvmContract, data []byte, account *EvmAccount) {
	deployTx := types.NewTransaction(account.Nonce, contract.Address, big.NewInt(int64(value)), txParams.GasLimit, txParams.GasPrice, data)
	signedTxBuffer, err := account.signAndMarshalTx(deployTx)
	require.Nil(bct.t, err)

	bct.transactionInstance(instID, byzcoin.Arguments{
		{Name: "tx", Value: signedTxBuffer},
	})

	account.Nonce += 1
}

func Test_InvokeToken(t *testing.T) {
	log.LLvl1("test: ERC20Token")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	instID := bct.createInstance(byzcoin.Arguments{})

	erc20Contract, err := getSmartContract("ERC20Token")
	require.Nil(t, err)

	/*
		A, AKey := GenerateKeys()
		B, BKey := GenerateKeys()
	*/
	a, err := NewEvmAccount(
		"0x627306090abab3a6e1400e9345bc60c78a8bef57",
		"c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3")
	require.Nil(t, err)
	b, err := NewEvmAccount(
		"0xf17f52151ebef6c7334fad080c5704d77216b732",
		"ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f")
	require.Nil(t, err)

	bct.bank(instID, "credit", a.Address, b.Address)
	bct.deploy(instID, txParams, 0, erc20Contract, nil, a)

	data, err := erc20Contract.packMethod("transfer", b.Address, big.NewInt(100))
	require.Nil(t, err)
	bct.transact(instID, txParams, 0, *erc20Contract, data, a)

	bct.bank(instID, "display", a.Address, b.Address)

	data, err = erc20Contract.packMethod("transfer", a.Address, big.NewInt(101))
	require.Nil(t, err)
	bct.transact(instID, txParams, 0, *erc20Contract, data, b)

	bct.bank(instID, "display", a.Address, b.Address)
}

func Test_InvokeLoanContract(t *testing.T) {
	log.LLvl1("Deploying Loan Contract")
	//Preparing ledger
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	instID := bct.createInstance(byzcoin.Arguments{})

	// Fetch LoanContract ABI and bytecode
	loanContract, err := getSmartContract("LoanContract")
	require.Nil(t, err)

	// Fetch erc20 bytecode
	erc20Contract, err := getSmartContract("ERC20Token")
	require.Nil(t, err)

	/*
		A, AKey := GenerateKeys()
		B, Bkey := GenerateKeys()
	*/
	a, err := NewEvmAccount(
		"0x627306090abab3a6e1400e9345bc60c78a8bef57",
		"c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3")
	require.Nil(t, err)
	b, err := NewEvmAccount(
		"0xf17f52151ebef6c7334fad080c5704d77216b732",
		"ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f")
	require.Nil(t, err)

	bct.bank(instID, "credit", a.Address, b.Address)
	bct.bank(instID, "display", a.Address, b.Address)

	bct.deploy(instID, txParams, 0, erc20Contract, nil, a)
	log.LLvl1("erc20 deployed", erc20Contract)

	//Constructor LoanContract
	//constructor (uint256 _wantedAmount, uint256 _interest, uint256 _tokenAmount, string _tokenName, ERC20Token _tokenContractAddress, uint256 _length) public {
	data, err := loanContract.packConstructor(big.NewInt(1*1e18), big.NewInt(0), big.NewInt(10000), "TestCoin", erc20Contract.Address, big.NewInt(0))
	require.Nil(t, err)

	bct.deploy(instID, txParams, 0, loanContract, data, a)
	log.LLvl1("LoanContract deployed", loanContract)

	// Check if there are enough tokens
	data, err = loanContract.packMethod("checkTokens")
	require.Nil(t, err)
	bct.transact(instID, txParams, 0, *loanContract, data, a)
	log.LLvl1("check tokens passed")

	log.LLvl1("test avant lend")
	bct.bank(instID, "display", a.Address, b.Address, loanContract.Address)

	/*
		balanceOfTest, err := abiMethodPack(erc20ABI, "balanceOf", common.HexToAddress(A))
		require.Nil(t, err)
		nonceA = transact(nonceA, 0, string(balanceOfTest), erc20Address.Hex(), AKey)
		log.LLvl1("balance of test")

		/*
		log.LLvl1("transafering token from B which has no tokens")
		checkBalance, err := abiMethodPack(erc20ABI, "transfer", common.HexToAddress(A), big.NewInt(1))
		require.Nil(t, err)
		nonceB = transact(nonceB, 0, string(checkBalance), erc20Address.Hex(), Bkey)
		log.LLvl1("this should fail")
	*/

	//LEND
	data, err = loanContract.packMethod("lend")
	require.Nil(t, err)
	bct.transact(instID, txParams, 2*WeiPerEther, *loanContract, data, b)
	log.LLvl1("lend passed")

	bct.bank(instID, "display", a.Address, b.Address, loanContract.Address)

	//    function payback () public payable {
	//paybackData, err := abiMethodPack(erc20ABI, "payback")
	require.Nil(t, err)
	bct.transact(instID, txParams, 2*WeiPerEther, *loanContract, []byte{}, a)
	log.LLvl1("payback, curious of what this does :")

	bct.bank(instID, "display", a.Address, b.Address, loanContract.Address)
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

func (bct *bcTest) displayAccountInstance(instID byzcoin.InstanceID, args byzcoin.Arguments) {
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID:    instID,
			SignerCounter: []uint64{bct.ct},
			Invoke: &byzcoin.Invoke{
				Command: "display",
				Args:    args,
			},
		}},
	}
	bct.ct++
	ctx.Instructions[0].Invoke.ContractID = "bvm"
	// And we need to sign the instruction with the signer that has his
	// public key stored in the darc.
	require.NoError(bct.t, ctx.FillSignersAndSignWith(bct.signer))
	// Sending this transaction to ByzCoin does not directly include it in the
	// global state - first we must wait for the new block to be created.
	var err error
	_, err = bct.cl.AddTransactionAndWait(ctx, 30)
	require.Nil(bct.t, err)
}

func (bct *bcTest) creditAccountInstance(instID byzcoin.InstanceID, args byzcoin.Arguments) {
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID:    instID,
			SignerCounter: []uint64{bct.ct},
			Invoke: &byzcoin.Invoke{
				Command: "credit",
				Args:    args,
			},
		}},
	}
	bct.ct++
	ctx.Instructions[0].Invoke.ContractID = "bvm"
	// And we need to sign the instruction with the signer that has his
	// public key stored in the darc.
	require.NoError(bct.t, ctx.FillSignersAndSignWith(bct.signer))

	// Sending this transaction to ByzCoin does not directly include it in the
	// global state - first we must wait for the new block to be created.
	var err error
	_, err = bct.cl.AddTransactionAndWait(ctx, 30)
	require.Nil(bct.t, err)
}

func (bct *bcTest) transactionInstance(instID byzcoin.InstanceID, args byzcoin.Arguments) {
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID:    instID,
			SignerCounter: []uint64{bct.ct},
			Invoke: &byzcoin.Invoke{
				Command: "transaction",
				Args:    args,
			},
		}},
	}
	bct.ct++
	ctx.Instructions[0].Invoke.ContractID = "bvm"
	// And we need to sign the instruction with the signer that has his
	// public key stored in the darc.
	require.NoError(bct.t, ctx.FillSignersAndSignWith(bct.signer))

	// Sending this transaction to ByzCoin does not directly include it in the
	// global state - first we must wait for the new block to be created.
	var err error
	_, err = bct.cl.AddTransactionAndWait(ctx, 30)
	require.Nil(bct.t, err)
}
