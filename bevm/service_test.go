package bevm

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func setupTest() (*onet.LocalTest, *onet.Roster, *Client) {
	local := onet.NewTCPTest(cothority.Suite)

	// Generate 1 host, don't connect, process messages, don't register the
	// tree or roster
	_, ro, _ := local.GenTree(1, false)

	client := &Client{Client: local.NewClient(ServiceName)}

	return local, ro, client
}

// -----------------------------------------------------------------------

func TestService_InputArgs(t *testing.T) {
	methodName := "method"

	abiJSON := `[{"constant":true,` +
		`"name":"` + methodName + `",` +
		`"inputs":[` +
		`{"name":"name1","type":"uint256"},` +
		`{"name":"name2","type":"address"}` +
		`],` +
		`"outputs":[{"name":"","type":"uint256"}],` +
		`"payable":false,"stateMutability":"view","type":"function"}]`
	testABI, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)

	argsNative := []interface{}{
		"100",
		"0x000102030405060708090a0b0c0d0e0f10111213",
	}
	arg1, err := strconv.ParseInt(argsNative[0].(string), 0, 64)
	require.NoError(t, err)
	expectedArgs := []interface{}{
		big.NewInt(arg1),
		common.HexToAddress(argsNative[1].(string)),
	}

	argsJSON := make([]string, len(argsNative))
	for i, arg := range argsNative {
		argJSON, err := json.Marshal(arg)
		require.NoError(t, err)
		argsJSON[i] = string(argJSON)
	}

	// Check that decoding does not fail ...
	args, err := DecodeEvmArgs(argsJSON, testABI.Methods[methodName].Inputs)
	require.NoError(t, err)

	// ... and produces the right arguments ...
	require.Equal(t, expectedArgs, args)

	// ... which are accepted by Ethereum
	_, err = testABI.Pack(methodName, args...)
	require.NoError(t, err)

	// Check that argument types which are not supported trigger an error
	abiJSON = `[{"constant":true,` +
		`"name":"` + methodName + `",` +
		`"inputs":[` +
		`{"name":"name2","type":"uint42"}` +
		`],` +
		`"outputs":[{"name":"","type":"uint256"}],` +
		`"payable":false,"stateMutability":"view","type":"function"}]`
	testABI, err = abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)

	args, err = DecodeEvmArgs([]string{`100`},
		testABI.Methods[methodName].Inputs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported type")
	require.Contains(t, err.Error(), "uint42")
}

func TestService_Deploy(t *testing.T) {
	local, ro, client := setupTest()
	defer local.CloseAll()

	log.Lvl1("Sending request to service...")

	// Deploy a Candy contract with 100 candies.
	// The expected values are taken from an execution using the BEvmClient.

	candyBytecode, err := hex.DecodeString("608060405234801561001057600080fd" +
		"5b506040516020806101cb833981018060405281019080805190602001909291" +
		"9050505080600081905550806001819055506000600281905550506101728061" +
		"00596000396000f30060806040526004361061004c576000357c010000000000" +
		"0000000000000000000000000000000000000000000000900463ffffffff1680" +
		"63a1ff2f5214610051578063ea319f281461007e575b600080fd5b3480156100" +
		"5d57600080fd5b5061007c600480360381019080803590602001909291905050" +
		"506100a9565b005b34801561008a57600080fd5b5061009361013c565b604051" +
		"8082815260200191505060405180910390f35b60015481111515156101235760" +
		"40517f08c379a000000000000000000000000000000000000000000000000000" +
		"00000081526004018080602001828103825260058152602001807f6572726f72" +
		"0000000000000000000000000000000000000000000000000000008152506020" +
		"0191505060405180910390fd5b80600154036001819055508060025401600281" +
		"90555050565b60006001549050905600a165627a7a723058207721a45f17c0e0" +
		"f57e255f33575281d17f1a90d3d58b51688230d93c460a19aa0029")
	require.NoError(t, err)

	candyAbi := `[{"constant":false,"inputs":[{"name":"candies","type":"uint` +
		`256"}],"name":"eatCandy","outputs":[],"payable":false,"stateMutabil` +
		`ity":"nonpayable","type":"function"},{"constant":true,"inputs":[],"` +
		`name":"getRemainingCandies","outputs":[{"name":"","type":"uint256"}` +
		`],"payable":false,"stateMutability":"view","type":"function"},{"inp` +
		`uts":[{"name":"_candies","type":"uint256"}],"payable":false,"stateM` +
		`utability":"nonpayable","type":"constructor"}]`

	candySupply, err := json.Marshal("100")
	require.NoError(t, err)

	response, err := client.PrepareDeployTx(ro.List[0], 1e7, 1, 0, 0,
		candyBytecode, candyAbi, string(candySupply))
	require.NoError(t, err)

	expectedTx, err := hex.DecodeString("7b226e6f6e6365223a22307830222c22" +
		"6761735072696365223a22307831222c22676173223a22307839383936383022" +
		"2c22746f223a6e756c6c2c2276616c7565223a22307830222c22696e70757422" +
		"3a22307836303830363034303532333438303135363130303130353736303030" +
		"3830666435623530363034303531363032303830363130316362383333393831" +
		"3031383036303430353238313031393038303830353139303630323030313930" +
		"3932393139303530353035303830363030303831393035353530383036303031" +
		"3831393035353530363030303630303238313930353535303530363130313732" +
		"3830363130303539363030303339363030306633303036303830363034303532" +
		"3630303433363130363130303463353736303030333537633031303030303030" +
		"3030303030303030303030303030303030303030303030303030303030303030" +
		"3030303030303030303030303030303030303930303436336666666666666666" +
		"3136383036336131666632663532313436313030353135373830363365613331" +
		"3966323831343631303037653537356236303030383066643562333438303135" +
		"3631303035643537363030303830666435623530363130303763363030343830" +
		"3336303338313031393038303830333539303630323030313930393239313930" +
		"3530353035303631303061393536356230303562333438303135363130303861" +
		"3537363030303830666435623530363130303933363130313363353635623630" +
		"3430353138303832383135323630323030313931353035303630343035313830" +
		"3931303339306633356236303031353438313131313531353135363130313233" +
		"3537363034303531376630386333373961303030303030303030303030303030" +
		"3030303030303030303030303030303030303030303030303030303030303030" +
		"3030303030303030303038313532363030343031383038303630323030313832" +
		"3831303338323532363030353831353236303230303138303766363537323732" +
		"3666373230303030303030303030303030303030303030303030303030303030" +
		"3030303030303030303030303030303030303030303030303030383135323530" +
		"3630323030313931353035303630343035313830393130333930666435623830" +
		"3630303135343033363030313831393035353530383036303032353430313630" +
		"3032383139303535353035303536356236303030363030313534393035303930" +
		"3536303061313635363237613761373233303538323037373231613435663137" +
		"6330653066353765323535663333353735323831643137663161393064336435" +
		"3862353136383832333064393363343630613139616130303239303030303030" +
		"3030303030303030303030303030303030303030303030303030303030303030" +
		"3030303030303030303030303030303030303030303030303634222c2276223a" +
		"22307830222c2272223a22307830222c2273223a22307830222c226861736822" +
		"3a22307837666631383834633430633664636561653534666361346331356131" +
		"3330633561336636393730326434663365373566653361633738623137356563" +
		"39356139227d")
	require.NoError(t, err)

	expectedHash, err := hex.DecodeString(
		"c289e67875d147429d2ffc5cc58e9a1486d581bef5aeca63017ad7855f8dab26")
	require.NoError(t, err)

	require.Equal(t, expectedTx, response.Transaction)
	require.Equal(t, expectedHash, response.TransactionHash)
}

func TestService_Transaction(t *testing.T) {
	local, ro, client := setupTest()
	defer local.CloseAll()

	log.Lvl1("Sending request to service...")

	// Call eatCandy(10) on a Candy contract deployed at
	// 0x8cdaf0cd259887258bc13a92c0a6da92698644c0.  The expected values are
	// taken from an execution using the BEvmClient.

	contractAddress, err := hex.DecodeString(
		"8cdaf0cd259887258bc13a92c0a6da92698644c0")
	require.NoError(t, err)

	candyAbi := `[{"constant":false,"inputs":[{"name":"candies","type":"uint2` +
		`56"}],"name":"eatCandy","outputs":[],"payable":false,"stateMutabilit` +
		`y":"nonpayable","type":"function"},{"constant":true,"inputs":[],"nam` +
		`e":"getRemainingCandies","outputs":[{"name":"","type":"uint256"}],"p` +
		`ayable":false,"stateMutability":"view","type":"function"},{"inputs":` +
		`[{"name":"_candies","type":"uint256"}],"payable":false,"stateMutabil` +
		`ity":"nonpayable","type":"constructor"}]`

	candiesToEat, err := json.Marshal("10")
	require.NoError(t, err)

	nonce := uint64(1) // First call right after deployment

	response, err := client.PrepareTransactionTx(ro.List[0], 1e7, 1, 0,
		contractAddress, nonce, candyAbi, "eatCandy", string(candiesToEat))
	require.NoError(t, err)

	expectedTx, err := hex.DecodeString("7b226e6f6e6365223a22307831222c22" +
		"6761735072696365223a22307831222c22676173223a22307839383936383022" +
		"2c22746f223a2230783863646166306364323539383837323538626331336139" +
		"3263306136646139323639383634346330222c2276616c7565223a2230783022" +
		"2c22696e707574223a2230786131666632663532303030303030303030303030" +
		"3030303030303030303030303030303030303030303030303030303030303030" +
		"3030303030303030303030303030303030303061222c2276223a22307830222c" +
		"2272223a22307830222c2273223a22307830222c2268617368223a2230786534" +
		"3264343137386465303032323636386433326637383033666564353637376437" +
		"343666393238666465386430656339303532656432306138616466343362227d")
	require.NoError(t, err)

	expectedHash, err := hex.DecodeString(
		"e13b1cfe8797fa11bd7929158008033e585d302a6f4cb11cfcf2b0a8bebec3fd")
	require.NoError(t, err)

	require.Equal(t, expectedTx, response.Transaction)
	require.Equal(t, expectedHash, response.TransactionHash)
}

func TestService_FinalizeTx(t *testing.T) {
	local, ro, client := setupTest()
	defer local.CloseAll()

	log.Lvl1("Sending request to service...")

	// Finalize a transaction combining the unsigned transaction and the
	// signature.  The expected values are taken from an execution using the
	// BEvmClient.

	// Unsigned transaction of Candy.eatCandy(10) (see
	// TestService_Transaction())
	unsignedTx, err := hex.DecodeString("7b226e6f6e6365223a22307831222c22" +
		"6761735072696365223a22307831222c22676173223a22307839383936383022" +
		"2c22746f223a2230783863646166306364323539383837323538626331336139" +
		"3263306136646139323639383634346330222c2276616c7565223a2230783022" +
		"2c22696e707574223a2230786131666632663532303030303030303030303030" +
		"3030303030303030303030303030303030303030303030303030303030303030" +
		"3030303030303030303030303030303030303061222c2276223a22307830222c" +
		"2272223a22307830222c2273223a22307830222c2268617368223a2230786534" +
		"3264343137386465303032323636386433326637383033666564353637376437" +
		"343666393238666465386430656339303532656432306138616466343362227d")
	require.NoError(t, err)

	// Signature done with private key
	// 0xc87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3
	signature, err := hex.DecodeString("aa0b243e4ad97b6cb7c2a016567aa02b2" +
		"e7bed159c221b7089b60688527f6e88679c9dfcb1ceb2477a36753645b564c2a" +
		"14a7bc757f46b9b714c49a4c93ea0a401")
	require.NoError(t, err)

	expectedTx, err := hex.DecodeString("7b226e6f6e6365223a22307831222c22" +
		"6761735072696365223a22307831222c22676173223a22307839383936383022" +
		"2c22746f223a2230783863646166306364323539383837323538626331336139" +
		"3263306136646139323639383634346330222c2276616c7565223a2230783022" +
		"2c22696e707574223a2230786131666632663532303030303030303030303030" +
		"3030303030303030303030303030303030303030303030303030303030303030" +
		"3030303030303030303030303030303030303061222c2276223a223078316322" +
		"2c2272223a223078616130623234336534616439376236636237633261303136" +
		"3536376161303262326537626564313539633232316237303839623630363838" +
		"3532376636653838222c2273223a223078363739633964666362316365623234" +
		"3737613336373533363435623536346332613134613762633735376634366239" +
		"6237313463343961346339336561306134222c2268617368223a223078346339" +
		"6633613434336166303032643837383966623561623939326137663134663939" +
		"6134303762616532613332643464653830313037366365613065353631227d")
	require.NoError(t, err)

	response, err := client.FinalizeTx(ro.List[0], unsignedTx, signature)
	require.NoError(t, err)

	require.Equal(t, expectedTx, response.Transaction)
}

func TestService_Call(t *testing.T) {
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
	candyAbi := getContractData(t, "Candy", "abi")
	candyBytecode := getContractData(t, "Candy", "bin")
	candySupply := big.NewInt(1103)
	candyContract, err := NewEvmContract(
		"Candy", candyAbi, candyBytecode)
	require.NoError(t, err)
	_, candyInstance, err := bevmClient.Deploy(
		txParams.GasLimit, txParams.GasPrice, 0, a, candyContract,
		candySupply)
	require.NoError(t, err)

	// Retrieve server TOML config
	grp := &app.Group{Roster: bct.roster}
	grpToml, err := grp.Toml(cothority.Suite)
	require.NoError(t, err)

	// Get remaining candies
	resp, err := bevmClient.PerformCall(
		bct.roster.List[0],
		bct.cl.ID,
		grpToml.String(),
		bevmClient.instanceID,
		a.Address[:],
		candyInstance.Address[:],
		candyAbi,
		"getRemainingCandies",
	)
	require.NoError(t, err)

	expectedResult, err := json.Marshal(candySupply)
	require.NoError(t, err)
	require.Equal(t, resp.Result, string(expectedResult))
}
