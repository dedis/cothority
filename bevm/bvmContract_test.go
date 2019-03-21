package bevm

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3/log"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/protobuf"
)

const WeiPerEther = 1e18

//Spawn a bvm
func Test_Spawn(t *testing.T) {
	log.LLvl1("test: instantiating evm")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	bct.createInstance(t, byzcoin.Arguments{})
}

//Credits and displays an account balance
func Test_InvokeCredit(t *testing.T) {
	log.LLvl1("test: crediting and displaying an account balance")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	instID := bct.createInstance(t, byzcoin.Arguments{})

	// Credit an account
	address := []byte("0x2afd357E96a3aCbcd01615681C1D7e3398d5fb61")
	amount, err := protobuf.Encode(&AmountData{Ether: 3, Wei: .1415926535 * WeiPerEther})
	require.Nil(t, err)
	bct.creditAccountInstance(t, instID, byzcoin.Arguments{
		{Name: "address", Value: address},
		{Name: "amount", Value: amount},
	})

	// Display its balance
	bct.displayAccountInstance(t, instID, byzcoin.Arguments{
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
	instID := bct.createInstance(t, byzcoin.Arguments{})

	addresses := [3]string{
		"0x627306090abab3a6e1400e9345bc60c78a8bef57",
		"0xf17f52151ebef6c7334fad080c5704d77216b732",
		"0xc5fdf4076b8f3a5357c5e395ab970b5b54098fef",
	}
	for i, addr := range addresses {
		address := []byte(addr)
		amount, err := protobuf.Encode(&AmountData{Ether: int64(i) + 1})
		require.Nil(t, err)

		bct.creditAccountInstance(t, instID, byzcoin.Arguments{
			{Name: "address", Value: address},
			{Name: "amount", Value: amount},
		})

		bct.displayAccountInstance(t, instID, byzcoin.Arguments{
			{Name: "address", Value: address},
		})
	}
}

func Test_InvokeToken(t *testing.T) {
	log.LLvl1("test: ERC20Token")

	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	// Spawn a new BEVM instance
	instID := bct.createInstance(t, byzcoin.Arguments{})

	erc20ABI, erc20Bytecode := getSmartContract("ERC20Token")

	/*
		A, AKey := GenerateKeys()
		B, BKey := GenerateKeys()
	*/
	A, AKey := "0x627306090abab3a6e1400e9345bc60c78a8bef57", "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3"
	B, BKey := "0xf17f52151ebef6c7334fad080c5704d77216b732", "ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f"
	nonceA, nonceB := uint64(0), uint64(0)

	//Getting transaction parameters
	gasLimit, gasPrice := transactionGasParameters()

	bank := func(instruction string, args ...string) {
		amount, err := protobuf.Encode(&AmountData{Ether: 5})
		require.Nil(t, err)

		for _, address := range args {
			switch instruction {
			case "credit":
				//Send credit instructions to Byzcoin and incrementing nonce counter
				bct.creditAccountInstance(t, instID, byzcoin.Arguments{
					{Name: "address", Value: []byte(address)},
					{Name: "amount", Value: amount},
				})
			case "display":
				bct.displayAccountInstance(t, instID, byzcoin.Arguments{
					{Name: "address", Value: []byte(address)},
				})
			default:
				log.LLvl1("incorrect instruction")
			}
		}
		if instruction == "credit" {
			log.LLvl1("credited", args, 5*1e18, "wei")
		}
	}

	deploy := func(nonce uint64, value uint64, data []byte, address common.Address, privateKey string) (uint64, common.Address) {
		deployTx := types.NewContractCreation(nonce, big.NewInt(int64(value)), gasLimit, gasPrice, data)
		signedTxBuffer, err := signAndMarshalTx(privateKey, deployTx)
		require.Nil(t, err)

		bct.transactionInstance(t, instID, byzcoin.Arguments{
			{Name: "tx", Value: signedTxBuffer},
		})

		//log.LLvl1("deployed new contract at", crypto.CreateAddress(common.HexToAddress(A), deployTx.Nonce()).Hex())
		//log.LLvl1("nonce tx", deployTx.Nonce(), "should check", nonce)

		contractAddress := crypto.CreateAddress(address, nonce)

		return nonce + 1, contractAddress
	}

	transact := func(nonce uint64, value uint64, data []byte, contractAddress string, privateKey string) uint64 {
		deployTx := types.NewTransaction(nonce, common.HexToAddress(contractAddress), big.NewInt(int64(value)), gasLimit, gasPrice, data)
		signedTxBuffer, err := signAndMarshalTx(privateKey, deployTx)
		require.Nil(t, err)

		bct.transactionInstance(t, instID, byzcoin.Arguments{
			{Name: "tx", Value: signedTxBuffer},
		})

		return nonce + 1
	}

	bank("credit", A, B)
	nonceA, erc20Address := deploy(nonceA, 0, common.Hex2Bytes(erc20Bytecode), common.HexToAddress(A), AKey)

	transferData, err := abiMethodPack(erc20ABI, "transfer", common.HexToAddress(B), big.NewInt(100))
	require.Nil(t, err)
	nonceA = transact(nonceA, 0, transferData, erc20Address.Hex(), AKey)

	bank("display", A, B)

	transferData, err = abiMethodPack(erc20ABI, "transfer", common.HexToAddress(A), big.NewInt(101))
	require.Nil(t, err)
	nonceB = transact(nonceB, 0, transferData, erc20Address.Hex(), BKey)

	bank("display", A, B)
}

func TestInvoke_LoanContract(t *testing.T) {
	log.LLvl1("Deploying Loan Contract")
	//Preparing ledger
	bct := newBCTest(t)
	bct.local.Check = onet.CheckNone
	defer bct.Close()

	//Instantiating evm
	args := byzcoin.Arguments{}
	instID := bct.createInstance(t, args)

	// Get the proof from byzcoin
	reply, err := bct.cl.GetProof(instID.Slice())
	require.Nil(t, err)
	// Make sure the proof is a matching proof and not a proof of absence.
	pr := reply.Proof
	require.True(t, pr.InclusionProof.Match(instID.Slice()))
	_, err = bct.cl.WaitProof(instID, bct.gMsg.BlockInterval, nil)
	require.Nil(t, err)

	//Fetch LoanContract bytecode and abi
	lcABI, lcBIN := getSmartContract("LoanContract")

	//Fetch erc20 bytecode and abi
	_, erc20Bytecode := getSmartContract("ERC20Token")

	/*
		A, AKey := GenerateKeys()
		B, Bkey := GenerateKeys()
	*/

	A, AKey := "0x627306090abab3a6e1400e9345bc60c78a8bef57", "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3"
	B, Bkey := "0xf17f52151ebef6c7334fad080c5704d77216b732", "ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f"
	nonceA, nonceB := uint64(0), uint64(0)

	//Getting transaction parameters
	gasLimit, gasPrice := transactionGasParameters()

	bank := func(instruction string, args ...string) {
		for _, address := range args {
			argsA := byzcoin.Arguments{{Name: "address", Value: []byte(address)}}
			switch instruction {
			case "credit":
				//Send credit instructions to Byzcoin and incrementing nonce counter
				bct.creditAccountInstance(t, instID, argsA)
				// Get the proof from byzcoin
				reply, err = bct.cl.GetProof(instID.Slice())
				require.Nil(t, err)
				// Make sure the proof is a matching proof and not a proof of absence.
				pr = reply.Proof
				require.True(t, pr.InclusionProof.Match(instID.Slice()))
				_, err = bct.cl.WaitProof(instID, bct.gMsg.BlockInterval, nil)
				require.Nil(t, err)
			case "display":
				bct.displayAccountInstance(t, instID, argsA)
				// Get the proof from byzcoin
				reply, err = bct.cl.GetProof(instID.Slice())
				require.Nil(t, err)
				// Make sure the proof is a matching proof and not a proof of absence.
				pr = reply.Proof
				require.True(t, pr.InclusionProof.Match(instID.Slice()))
				_, err = bct.cl.WaitProof(instID, bct.gMsg.BlockInterval, nil)
				require.Nil(t, err)
			default:
				log.LLvl1("incorrect instruction")
			}
		}
		if instruction == "credit" {
			log.LLvl1("credited", args, 5*1e18, "wei")
		}
	}

	deploy := func(nonce uint64, value uint64, data string, privateKey string) uint64 {
		deployTx := types.NewContractCreation(nonce, big.NewInt(int64(value)), gasLimit, gasPrice, common.Hex2Bytes(data))
		signedTxBuffer, err := signAndMarshalTx(privateKey, deployTx)
		require.Nil(t, err)
		args = byzcoin.Arguments{
			{
				Name:  "tx",
				Value: signedTxBuffer,
			},
		}
		bct.transactionInstance(t, instID, args)
		// Get the proof from byzcoin
		reply, err = bct.cl.GetProof(instID.Slice())
		require.Nil(t, err)
		// Make sure the proof is a matching proof and not a proof of absence.
		pr = reply.Proof
		require.True(t, pr.InclusionProof.Match(instID.Slice()))

		_, err = bct.cl.WaitProof(instID, bct.gMsg.BlockInterval, nil)
		require.Nil(t, err)

		//log.LLvl1("deployed new contract at", crypto.CreateAddress(common.HexToAddress(A), deployTx.Nonce()).Hex())
		//log.LLvl1("nonce tx", deployTx.Nonce(), "should check", nonce)
		return nonce + 1
	}

	transact := func(nonce uint64, value uint64, data string, contractAddress string, privateKey string) uint64 {
		deployTx := types.NewTransaction(nonce, common.HexToAddress(contractAddress), big.NewInt(int64(value)), gasLimit, gasPrice, []byte(data))
		signedTxBuffer, err := signAndMarshalTx(privateKey, deployTx)
		require.Nil(t, err)
		args = byzcoin.Arguments{
			{
				Name:  "tx",
				Value: signedTxBuffer,
			},
		}
		bct.transactionInstance(t, instID, args)
		// Get the proof from byzcoin
		reply, err = bct.cl.GetProof(instID.Slice())
		require.Nil(t, err)
		// Make sure the proof is a matching proof and not a proof of absence.
		pr = reply.Proof
		require.True(t, pr.InclusionProof.Match(instID.Slice()))
		_, err = bct.cl.WaitProof(instID, bct.gMsg.BlockInterval, nil)
		require.Nil(t, err)
		return nonce + 1
	}

	//bank("display", A, B)
	bank("credit", A, B)
	//bank("display", A, B)

	nonceA = deploy(nonceA, 0, erc20Bytecode, AKey)
	erc20Address := crypto.CreateAddress(common.HexToAddress(A), nonceA-1)
	log.LLvl1("erc20 deployed @", erc20Address.Hex())

	//Constructor LoanContract
	//constructor (uint256 _wantedAmount, uint256 _interest, uint256 _tokenAmount, string _tokenName, ERC20Token _tokenContractAddress, uint256 _length) public {
	constructorData, err := abiMethodPack(lcABI, "", big.NewInt(1*1e18), big.NewInt(0), big.NewInt(10000), "TestCoin", erc20Address, big.NewInt(0))
	require.Nil(t, err)
	s := []string{}
	s = append(s, lcBIN)
	encodedArgs := common.Bytes2Hex(constructorData)
	s = append(s, encodedArgs)
	lcData := strings.Join(s, "")

	nonceA = deploy(nonceA, 0, lcData, AKey)
	loanContractAddress := crypto.CreateAddress(common.HexToAddress(A), nonceA-1)
	log.LLvl1("LoanContract deployed @", loanContractAddress.Hex())

	//Check if there are enough tokens
	checkTokenData, err := abiMethodPack(lcABI, "checkTokens")
	require.Nil(t, err)
	nonceA = transact(nonceA, 0, string(checkTokenData), loanContractAddress.Hex(), AKey)
	log.LLvl1("check tokens passed")

	log.LLvl1("test avant lend")
	bank("display", loanContractAddress.Hex())

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
	lendData, err := abiMethodPack(lcABI, "lend")
	require.Nil(t, err)
	nonceB = transact(nonceB, 2*1e18, string(lendData), loanContractAddress.Hex(), Bkey)
	log.LLvl1("lend passed")

	bank("display", A)
	bank("display", B)
	bank("display", loanContractAddress.Hex())

	//    function payback () public payable {
	//paybackData, err := abiMethodPack(erc20ABI, "payback")
	require.Nil(t, err)
	nonceA = transact(nonceA, 2*1e18, "", loanContractAddress.Hex(), AKey)
	log.LLvl1("payback, curious of what this does :")

	bank("display", A)
	bank("display", B)
	bank("display", loanContractAddress.Hex())
}

//Signs the transaction with a private key and returns the transaction in byte format, ready to be included into the Byzcoin transaction
func signAndMarshalTx(privateKey string, tx *types.Transaction) ([]byte, error) {
	private, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, err
	}
	var signer types.Signer = types.HomesteadSigner{}
	signedTx, err := types.SignTx(tx, signer, private)
	if err != nil {
		return nil, err
	}
	signedBuffer, err := signedTx.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return signedBuffer, err
}

//Creates the data to interact with an existing contract, with a variadic number of arguments
func abiMethodPack(contractABI string, methodCall string, args ...interface{}) (data []byte, err error) {
	ABI, err := abi.JSON(strings.NewReader(contractABI))
	if err != nil {
		return nil, err
	}
	abiCall, err := ABI.Pack(methodCall, args...)
	if err != nil {
		log.LLvl1("error in packing args", err)
		return nil, err
	}
	return abiCall, nil
}

//Return gas parameters for easy modification
func transactionGasParameters() (gasLimit uint64, gasPrice *big.Int) {
	gasLimit = uint64(1e7)
	gasPrice = big.NewInt(1)
	return
}

// bcTest is used here to provide some simple test structure for different
// tests.
type bcTest struct {
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
	out = &bcTest{}
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

func (bct *bcTest) createInstance(t *testing.T, args byzcoin.Arguments) byzcoin.InstanceID {
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
	require.NoError(t, ctx.FillSignersAndSignWith(bct.signer))

	// Sending this transaction to ByzCoin does not directly include it in the
	// global state - first we must wait for the new block to be created.
	var err error
	_, err = bct.cl.AddTransactionAndWait(ctx, 20)
	require.Nil(t, err)
	return ctx.Instructions[0].DeriveID("")
}

func (bct *bcTest) displayAccountInstance(t *testing.T, instID byzcoin.InstanceID, args byzcoin.Arguments) {
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
	require.NoError(t, ctx.FillSignersAndSignWith(bct.signer))
	// Sending this transaction to ByzCoin does not directly include it in the
	// global state - first we must wait for the new block to be created.
	var err error
	_, err = bct.cl.AddTransactionAndWait(ctx, 30)
	require.Nil(t, err)
}

func (bct *bcTest) creditAccountInstance(t *testing.T, instID byzcoin.InstanceID, args byzcoin.Arguments) {
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
	require.NoError(t, ctx.FillSignersAndSignWith(bct.signer))

	// Sending this transaction to ByzCoin does not directly include it in the
	// global state - first we must wait for the new block to be created.
	var err error
	_, err = bct.cl.AddTransactionAndWait(ctx, 30)
	require.Nil(t, err)
}

func (bct *bcTest) transactionInstance(t *testing.T, instID byzcoin.InstanceID, args byzcoin.Arguments) {
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
	require.NoError(t, ctx.FillSignersAndSignWith(bct.signer))

	// Sending this transaction to ByzCoin does not directly include it in the
	// global state - first we must wait for the new block to be created.
	var err error
	_, err = bct.cl.AddTransactionAndWait(ctx, 30)
	require.Nil(t, err)
}
