package bevm

import (
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

func TestService_Call(t *testing.T) {
	// Create a new ledger and prepare for proper closing
	bct := newBCTest(t)
	defer bct.CloseAll()

	// Spawn a new BEvm instance
	instanceID, err := NewBEvm(bct.Client, bct.Signer, bct.GenesisDarc)
	require.NoError(t, err)

	// Create a new BEvm client
	bevmClient, err := NewClient(bct.Client, bct.Signer, instanceID)
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

	// Ensure transaction is propagated to all nodes
	require.NoError(t, bct.Client.WaitPropagation(-1))

	callData, err := candyInstance.packMethod("getRemainingCandies")
	require.NoError(t, err)

	// Get remaining candies
	resp, err := bevmClient.viewCall(
		bct.Roster.List[0],
		bct.Client.ID,
		bevmClient.instanceID,
		a.Address[:],
		candyInstance.Address[:],
		callData,
	)
	require.NoError(t, err)

	expectedResult, err := candyContract.Abi.Methods["getRemainingCandies"].
		Outputs.Pack(candySupply)
	require.NoError(t, err)

	require.Equal(t, resp.Result, expectedResult)
}
