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
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func setupTest() (*onet.LocalTest, *onet.Roster, *Client) {
	local := onet.NewTCPTest(tSuite)

	// Generate 1 host, don't connect, process messages, don't register the
	// tree or entitylist
	_, ro, _ := local.GenTree(1, false)

	client := &Client{onetClient: local.NewClient(ServiceName)}

	// Do not check leaking goroutines (Ethereum leaves goroutines running...)
	local.Check = onet.CheckNone

	return local, ro, client
}

func teardownTest(local *onet.LocalTest) {
	local.CloseAll()
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

	args, err = DecodeEvmArgs([]string{`100`}, testABI.Methods[methodName].Inputs)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "unsupported type")
	require.Contains(t, err.Error(), "uint42")
}

func TestService_Deploy(t *testing.T) {
	local, ro, client := setupTest()
	defer teardownTest(local)

	log.Lvl1("Sending request to service...")

	// Deploy a Candy contract with 100 candies.
	// The expected values are taken from an execution using the BEvmClient.

	candyBytecode, err := hex.DecodeString("608060405234801561001057600080fd5b506040516020806101cb833981018060405281019080805190602001909291905050508060008190555080600181905550600060028190555050610172806100596000396000f30060806040526004361061004c576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063a1ff2f5214610051578063ea319f281461007e575b600080fd5b34801561005d57600080fd5b5061007c600480360381019080803590602001909291905050506100a9565b005b34801561008a57600080fd5b5061009361013c565b6040518082815260200191505060405180910390f35b6001548111151515610123576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260058152602001807f6572726f7200000000000000000000000000000000000000000000000000000081525060200191505060405180910390fd5b8060015403600181905550806002540160028190555050565b60006001549050905600a165627a7a723058207721a45f17c0e0f57e255f33575281d17f1a90d3d58b51688230d93c460a19aa0029")
	require.NoError(t, err)

	candyAbi := `[{"constant":false,"inputs":[{"name":"candies","type":"uint256"}],"name":"eatCandy","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"getRemainingCandies","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"inputs":[{"name":"_candies","type":"uint256"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"}]`

	candySupply, err := json.Marshal("100")
	require.NoError(t, err)

	response, err := client.PrepareDeployTx(ro.List[0], 1e7, 1, 0, 0, candyBytecode, candyAbi, string(candySupply))
	require.NoError(t, err)

	expectedTx, err := hex.DecodeString("7b226e6f6e6365223a22307830222c226761735072696365223a22307831222c22676173223a223078393839363830222c22746f223a6e756c6c2c2276616c7565223a22307830222c22696e707574223a22307836303830363034303532333438303135363130303130353736303030383066643562353036303430353136303230383036313031636238333339383130313830363034303532383130313930383038303531393036303230303139303932393139303530353035303830363030303831393035353530383036303031383139303535353036303030363030323831393035353530353036313031373238303631303035393630303033393630303066333030363038303630343035323630303433363130363130303463353736303030333537633031303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303039303034363366666666666666663136383036336131666632663532313436313030353135373830363365613331396632383134363130303765353735623630303038306664356233343830313536313030356435373630303038306664356235303631303037633630303438303336303338313031393038303830333539303630323030313930393239313930353035303530363130306139353635623030356233343830313536313030386135373630303038306664356235303631303039333631303133633536356236303430353138303832383135323630323030313931353035303630343035313830393130333930663335623630303135343831313131353135313536313031323335373630343035313766303863333739613030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303831353236303034303138303830363032303031383238313033383235323630303538313532363032303031383037663635373237323666373230303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303038313532353036303230303139313530353036303430353138303931303339306664356238303630303135343033363030313831393035353530383036303032353430313630303238313930353535303530353635623630303036303031353439303530393035363030613136353632376137613732333035383230373732316134356631376330653066353765323535663333353735323831643137663161393064336435386235313638383233306439336334363061313961613030323930303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303634222c2276223a22307830222c2272223a22307830222c2273223a22307830222c2268617368223a22307837666631383834633430633664636561653534666361346331356131333063356133663639373032643466336537356665336163373862313735656339356139227d")
	require.NoError(t, err)

	expectedHash, err := hex.DecodeString("c289e67875d147429d2ffc5cc58e9a1486d581bef5aeca63017ad7855f8dab26")
	require.NoError(t, err)

	require.Equal(t, expectedTx, response.Transaction)
	require.Equal(t, expectedHash, response.TransactionHash)
}

func TestService_Transaction(t *testing.T) {
	local, ro, client := setupTest()
	defer teardownTest(local)

	log.Lvl1("Sending request to service...")

	// Call eatCandy(10) on a Candy contract deployed at 0x8cdaf0cd259887258bc13a92c0a6da92698644c0.
	// The expected values are taken from an execution using the BEvmClient.

	contractAddress, err := hex.DecodeString("8cdaf0cd259887258bc13a92c0a6da92698644c0")
	require.NoError(t, err)

	candyAbi := `[{"constant":false,"inputs":[{"name":"candies","type":"uint256"}],"name":"eatCandy","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"getRemainingCandies","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"inputs":[{"name":"_candies","type":"uint256"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"}]`

	candiesToEat, err := json.Marshal("10")
	require.NoError(t, err)

	nonce := uint64(1) // First call right after deployment

	response, err := client.PrepareTransactionTx(ro.List[0], 1e7, 1, 0, contractAddress, nonce, candyAbi, "eatCandy", string(candiesToEat))
	require.NoError(t, err)

	expectedTx, err := hex.DecodeString("7b226e6f6e6365223a22307831222c226761735072696365223a22307831222c22676173223a223078393839363830222c22746f223a22307838636461663063643235393838373235386263313361393263306136646139323639383634346330222c2276616c7565223a22307830222c22696e707574223a223078613166663266353230303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303061222c2276223a22307830222c2272223a22307830222c2273223a22307830222c2268617368223a22307865343264343137386465303032323636386433326637383033666564353637376437343666393238666465386430656339303532656432306138616466343362227d")
	require.NoError(t, err)

	expectedHash, err := hex.DecodeString("e13b1cfe8797fa11bd7929158008033e585d302a6f4cb11cfcf2b0a8bebec3fd")
	require.NoError(t, err)

	require.Equal(t, expectedTx, response.Transaction)
	require.Equal(t, expectedHash, response.TransactionHash)
}

func TestService_FinalizeTx(t *testing.T) {
	local, ro, client := setupTest()
	defer teardownTest(local)

	log.Lvl1("Sending request to service...")

	// Finalize a transaction combining the unsigned transaction and the signature.
	// The expected values are taken from an execution using the BEvmClient.

	// Unsigned transaction of Candy.eatCandy(10) (see TestService_Transaction())
	unsignedTx, err := hex.DecodeString("7b226e6f6e6365223a22307831222c226761735072696365223a22307831222c22676173223a223078393839363830222c22746f223a22307838636461663063643235393838373235386263313361393263306136646139323639383634346330222c2276616c7565223a22307830222c22696e707574223a223078613166663266353230303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303061222c2276223a22307830222c2272223a22307830222c2273223a22307830222c2268617368223a22307865343264343137386465303032323636386433326637383033666564353637376437343666393238666465386430656339303532656432306138616466343362227d")
	require.NoError(t, err)

	// Signature done with private key 0xc87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3
	signature, err := hex.DecodeString("aa0b243e4ad97b6cb7c2a016567aa02b2e7bed159c221b7089b60688527f6e88679c9dfcb1ceb2477a36753645b564c2a14a7bc757f46b9b714c49a4c93ea0a401")
	require.NoError(t, err)

	expectedTx, err := hex.DecodeString("7b226e6f6e6365223a22307831222c226761735072696365223a22307831222c22676173223a223078393839363830222c22746f223a22307838636461663063643235393838373235386263313361393263306136646139323639383634346330222c2276616c7565223a22307830222c22696e707574223a223078613166663266353230303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303061222c2276223a2230783163222c2272223a22307861613062323433653461643937623663623763326130313635363761613032623265376265643135396332323162373038396236303638383532376636653838222c2273223a22307836373963396466636231636562323437376133363735333634356235363463326131346137626337353766343662396237313463343961346339336561306134222c2268617368223a22307834633966336134343361663030326438373839666235616239393261376631346639396134303762616532613332643464653830313037366365613065353631227d")
	require.NoError(t, err)

	response, err := client.FinalizeTx(ro.List[0], unsignedTx, signature)
	require.NoError(t, err)

	require.Equal(t, expectedTx, response.Transaction)
}

func TestService_Call(t *testing.T) {
	t.Skip("Need cothority setup -- enable manually if really needed")

	local, ro, client := setupTest()
	defer teardownTest(local)

	log.Lvl1("Sending request to service...")

	// Call getRemainingCandies() on a Candy contract deployed at 0x8cdaf0cd259887258bc13a92c0a6da92698644c0.
	// The expected values are taken from an execution using the BEvmClient.

	bevmInstanceIDhex, err := hex.DecodeString("5aa2f1b11eb3f7f859bccf3cbd9d71dc1723f6978b1fee9bf4e88e78e13e89f1")
	require.NoError(t, err)
	bevmInstanceID := byzcoin.NewInstanceID(bevmInstanceIDhex)

	accountAddress, err := hex.DecodeString("627306090abab3a6e1400e9345bc60c78a8bef57")
	require.NoError(t, err)

	contractAddress, err := hex.DecodeString("8cdaf0cd259887258bc13a92c0a6da92698644c0")
	require.NoError(t, err)

	byzcoinID, err := hex.DecodeString("92601ebb9b0499efc5fde5141d0005f4a6fd350f39bff5a87efb8d4567091929")
	require.NoError(t, err)

	serverConfig := `[[servers]]
  Address = "tls://localhost:7776"
  Suite = "Ed25519"
  Public = "ed2494dfd826cd2c2ea23adedf564fb19619c6004bff91f08bc76e80bdb4ec7f"
  Description = "Conode_4"
  [servers.Services]
    [servers.Services.ByzCoin]
      Public = "01dc5f40cae57758c6e7200106d5784f6bcb668959ddfd2f702f6aed63e47e3a6d90a61899a315b6fccaec991a4f2807d4fedce0b53c125c2005d34e0c1b4a9478cf60c1e5ab24a1e4ab597f596b4e2ba06af19cc3e5589bda58030a0f70f8208abfeeb072e04a87c79f2f814634257be9b0be4f9b8b6a927abcdfab099bc16c"
      Suite = "bn256.adapter"
    [servers.Services.Skipchain]
      Public = "3ff215e1755712e28f8f4d60ca412101c60d3707a68f68b37cf6c29437cc315c79ab1190fa941309e50dee30eeb677e6f2b8796f01d99a866c24dd5dd59594840dd387970c6eaaf6b56c8f8055c7c9d65f3a82e1bfc3bb7efb80d5faa9c33ff35099a96c9dbd32e65e3448f78693d00b346400796629229432161e1044b0af5f"
      Suite = "bn256.adapter"
[[servers]]
  Address = "tls://localhost:7774"
  Suite = "Ed25519"
  Public = "0a0bdbb3f4059e9dad2d92b967bde211865f7d00839abd3330d8c9c4423b10bc"
  Description = "Conode_3"
  [servers.Services]
    [servers.Services.ByzCoin]
      Public = "6ea7db10d9f93b36045203d4008501f30a80d7c177847a784b483dcf6fdcfbe47e9f0123093ca3d715307662a642c684a3884656fc75c04d16f3cb1db67cd9e12f8c5ea637d124e1824522ce445f2848763bf3962b05ee662eafb78ac8ddd3b8771bccc8e920287857f56eabe094e5962f201a11f1f2c8ab388ff47dcb2e1f7a"
      Suite = "bn256.adapter"
    [servers.Services.Skipchain]
      Public = "58eaa4086f9033bb6398a8d4a6e6a7c136aa19e85c452f0ae069eb5a008e220305f726a056451ae0cb2c8deec820d6b5ad6585684122c38199403fa49bafeda06734432240cac370d70a5be9799258d044fb04f6aa634fed5d4c7080b340e08359142bbbd602323924ee97db1dbf6e3fb19b941880156cb98552fbe957115743"
      Suite = "bn256.adapter"
[[servers]]
  Address = "tls://localhost:7772"
  Suite = "Ed25519"
  Public = "5f1a868b2dfa1e799c958a2dd5d850a660e3596a5ceb4fe7ff9dcf9c2efd419b"
  Description = "Conode_2"
  [servers.Services]
    [servers.Services.ByzCoin]
      Public = "70208fdcbaa6f3fa539380d5b19d7318a1c8ae46aa8af1d17e2d321afbda46397654fd72432f2050689f3c942801fe9e2e401d73c1accae8b7f683c0a261c57469937eb409864b1d9c0ed5fd012ec0b4fa835b92c12770e5b3cd5b900528fa9b1b6672b9121d68b4f98fd238918c96c31643271d2ac0fdb54af15dabfd772f6c"
      Suite = "bn256.adapter"
    [servers.Services.Skipchain]
      Public = "7dafa5bc547beb1ecb26267df3b5294e1a641c356d1039cc5c94acc0048a56fb2e2d6dc7507291cf4fe03418e1e16f0810637a67e9a31edf8d06cca399f0f5c85e3dbe740bd564968467b0cc1792688791bd59a61eb98723ab30ab3f784e2225054437110ea972c43f633dc510fd07d50871ec346ee1c088e5441d415dd9e95e"
      Suite = "bn256.adapter"
[[servers]]
  Address = "tls://localhost:7770"
  Suite = "Ed25519"
  Public = "3de71200e7ecaeb49dc7f824317fb4ef6890e90018c49617139b6e61075f0247"
  Description = "Conode_1"
  [servers.Services]
    [servers.Services.ByzCoin]
      Public = "7ab3a36be090002cf36a82bc606d6b0ef1c4432abae0c432c0ab02c9c0d5b2513c6f18625f847aef2d49a57fe5adaea103ba48dc60e9b4dd51f1beecce2b0a2f763a25ca4e2a460b20fd3e80e0d9d306b760cd9c715ecbc77047e875f32dc8435ee5ceb8910a1290827d4fbf61483aa7758c81f83ab9a8ca58fc8a6b1c0f1d5b"
      Suite = "bn256.adapter"
    [servers.Services.Skipchain]
      Public = "0524681253b82af55c0976e792014707c39405fe215bb1ebf6a3159dcbbb944535619f32ed4a91a4d1fcf4d9aa4ad14a1d349d5354dbbd6fb51907087a09ce7862ee5808a4c3f5b3b23ee631f1ce42b56107acec13fa06817263d1e7f77938f1149249e598fd24207e7e5e33ece750d36fe966faf8fda9c7ace13a6a8b0b9fa4"
      Suite = "bn256.adapter"
`

	candyAbi := `[{"constant":false,"inputs":[{"name":"candies","type":"uint256"}],"name":"eatCandy","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"getRemainingCandies","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"inputs":[{"name":"_candies","type":"uint256"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"}]`

	response, err := client.PerformCall(ro.List[0], byzcoinID, serverConfig, bevmInstanceID, accountAddress, contractAddress, candyAbi, "getRemainingCandies")
	require.NoError(t, err)

	var result interface{}
	err = json.Unmarshal([]byte(response.Result), &result)
	require.NoError(t, err)

	expectedResult := float64(55)
	require.Equal(t, expectedResult, result)
}
