package medco_test

import (
	"testing"
	"github.com/dedis/cothority/lib/testutil"
	_"github.com/dedis/cothority_clone/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"time"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/protocols/medco"
	"strconv"
	"github.com/dedis/crypto/random"
)

const NUM_MESS = 100
const needle = "code9"
func codeGen(i int) string {
	return "code" + strconv.Itoa(i%10)
}

func Test5Nodes(t *testing.T) {

	defer testutil.AfterTest(t)
	local := sda.NewLocalTest()
	nNodes := 5
	dbg.TestOutput(testing.Verbose(), 1)
	host,entityList, tree := local.GenTree(nNodes, false, true, true)
	defer local.CloseAll()

	root,_ := local.CreateNewNodeName("PrivateCount", tree)
	suite := host[0].Suite()
	aggregateKey := entityList.Aggregate

	dbg.Lvl1("Encrypting dummy data...")
	var targetCount int64
	var code string
	EncryptedData := make([]medco.CipherText, NUM_MESS, NUM_MESS)
	for i := 0; i < NUM_MESS; i++ {
		if code = codeGen(i); code  == needle {
			targetCount += 1
		}
		c, _ := medco.EncryptBytes(suite, aggregateKey, []byte(code))
		EncryptedData[i] = *c
	}

	clientSecret  := suite.Secret().Pick(random.Stream)
	clientPublic := suite.Point().Mul(suite.Point().Base(), clientSecret)
	clientQuery,_ := medco.EncryptBytes(suite, aggregateKey, []byte(needle))

	root.ProtocolInstance().(*medco.PrivateCountProtocol).EncryptedData = &EncryptedData
	root.ProtocolInstance().(*medco.PrivateCountProtocol).ClientPublicKey = &clientPublic
	root.ProtocolInstance().(*medco.PrivateCountProtocol).ClientQuery = clientQuery

	feedback := root.ProtocolInstance().(*medco.PrivateCountProtocol).FeedbackChannel

	go root.Start()

	timeout := network.WaitRetry * time.Duration(network.MaxRetry*nNodes*2) * time.Millisecond

	select {
	case encryptedResult := <- feedback:
		result := medco.DecryptInt(suite, clientSecret, encryptedResult)
		dbg.Lvl1("RESULT:")
		dbg.Lvl1("Done with result =",result, "(target", targetCount,")" )
		if targetCount != result {
			t.Fatal("Bad result.")
		}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}
