package ocs_test

import (
	"testing"

	// We need to include the service so it is started.
	"github.com/dedis/onchain-secrets"
	_ "github.com/dedis/onchain-secrets/service"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/cothority.v1/skipchain"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestClient_CreateSkipchain(t *testing.T) {
	doTest(t, 0)
}

func TestClient_WriteRequest(t *testing.T) {
	doTest(t, 1)
}

func TestClient_ReadRequest(t *testing.T) {
	doTest(t, 2)
}

func TestClient_DecryptKeyRequest(t *testing.T) {
	doTest(t, 3)
}

func TestClient_GetData(t *testing.T) {
	doTest(t, 4)
}

func TestClient_GetReadRequests(t *testing.T) {
	doTest(t, 5)
}

type testStruct struct {
	local  *onet.LocalTest
	roster *onet.Roster
	cl     *ocs.Client
	data   []byte
	reader *config.KeyPair
	sym    []byte
	scurl  *ocs.SkipChainURL
	write  *skipchain.SkipBlock
	read   *skipchain.SkipBlock
}

func doTest(t *testing.T, step int) {
	test := &testStruct{
		local: onet.NewTCPTest(),
		cl:    ocs.NewClient(),
	}
	defer test.local.CloseAll()
	_, test.roster, _ = test.local.GenTree(3, true)

	for i := 0; i <= step; i++ {
		switch i {
		case 0:
			createSkipchain(t, test)
		case 1:
			writeData(t, test)
		case 2:
			readData(t, test)
		case 3:
			decryptKey(t, test)
		case 4:
			getData(t, test)
		case 5:
			getReadRequests(t, test)
		}
	}
}

func createSkipchain(t *testing.T, test *testStruct) {
	var cerr onet.ClientError
	test.scurl, cerr = test.cl.CreateSkipchain(test.roster)
	log.ErrFatal(cerr)
}

func writeData(t *testing.T, test *testStruct) {
	var cerr onet.ClientError
	test.data = []byte("Very secret document")

	var enc []byte
	enc, test.sym = encryptDocument(test.data)
	test.reader = config.NewKeyPair(network.Suite)
	test.write, cerr = test.cl.WriteRequest(test.scurl, enc, test.sym, []abstract.Point{test.reader.Public})
	log.ErrFatal(cerr)

	dataOCS := ocs.NewDataOCS(test.write.Data)
	require.NotNil(t, dataOCS)
	require.NotNil(t, dataOCS.Write)
}

func readData(t *testing.T, test *testStruct) {
	var cerr onet.ClientError
	test.read, cerr = test.cl.ReadRequest(test.scurl, test.write.Hash, test.reader.Secret)
	log.ErrFatal(cerr)
}

func decryptKey(t *testing.T, test *testStruct) {
	var cerr onet.ClientError

	sym, cerr := test.cl.DecryptKeyRequest(test.scurl, test.read.Hash, test.reader.Secret)
	log.ErrFatal(cerr)
	require.Equal(t, test.sym, sym)
}

func getData(t *testing.T, test *testStruct) {
	encData, cerr := test.cl.GetData(test.scurl, test.write.Hash)
	log.ErrFatal(cerr)

	cipher := network.Suite.Cipher(test.sym)
	data, err := cipher.Open(nil, encData)
	log.ErrFatal(err)

	require.Equal(t, test.data, data)
}

func getReadRequests(t *testing.T, test *testStruct) {
	docs, cerr := test.cl.GetReadRequests(test.scurl, test.write.Hash, 0)
	log.ErrFatal(cerr)
	require.Equal(t, 1, len(docs))
	require.Equal(t, test.write.Hash, docs[0].DataID)
}

// EncryptDocument takes data and a credential, then it creates a new
// symmetric encryption key, encrypts the document, and stores the document and
// the encryption key on the blockchain.
func encryptDocument(data []byte) (encData, key []byte) {
	key = random.Bytes(128, random.Stream)
	cipher := network.Suite.Cipher(key)
	encData = cipher.Seal(nil, data)
	return
}
