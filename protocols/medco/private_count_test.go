package medco_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/crypto/random"
	"reflect"
	"strconv"
	"testing"
	"time"
)

const NUM_MESS = 5
const NUM_BUCKET = 1
var BUCKET_DESC = []int64{/*10,20,30,40,50,60,70, 80,90*/}
const needle = "code0"
func codeGen(i int) string {
	return "code" + strconv.Itoa(i%1)
}
func bucketGen(i int) int {
	return i % NUM_BUCKET
}

func Test5Nodes(t *testing.T) {
	defer dbg.AfterTest(t)
	local := sda.NewLocalTest()
	nNodes := 5
	dbg.TestOutput(testing.Verbose(), 1)
	host,entityList, tree := local.GenTree(nNodes, false, true, true)
	defer local.CloseAll()



	root,_ := local.CreateNewNodeName("PrivateCount", tree)
	suite := host[0].Suite()
	aggregateKey := entityList.Aggregate

	clientSecret  := suite.Secret().Pick(random.Stream)
	clientPublic := suite.Point().Mul(suite.Point().Base(), clientSecret)
	clientQuery,_ := medco.EncryptBytes(suite, aggregateKey, []byte(needle))

	dbg.Lvl1("Encrypting dummy data...")
	targetCounts := make([]int64, NUM_BUCKET)
	var code string
	EncryptedData := make([]medco.HolderResponseData, NUM_MESS, NUM_MESS)
	for i := 0; i < NUM_MESS; i++ {
		if code = codeGen(i); code  == needle {
			targetCounts[bucketGen(i)] += 1
		}
		c, _ := medco.EncryptBytes(suite, aggregateKey, []byte(code))
		buckets := medco.NullCipherVector(suite, NUM_BUCKET, clientPublic)
		buckets[bucketGen(i)] = *medco.EncryptInt(suite, clientPublic, 1)
		EncryptedData[i] = medco.HolderResponseData{buckets, *c}
	}
	dbg.Lvl1("... Done")


	root.ProtocolInstance().(*medco.PrivateCountProtocol).EncryptedData = &EncryptedData
	root.ProtocolInstance().(*medco.PrivateCountProtocol).ClientPublicKey = &clientPublic
	root.ProtocolInstance().(*medco.PrivateCountProtocol).ClientQuery = clientQuery
	root.ProtocolInstance().(*medco.PrivateCountProtocol).BucketDesc = &BUCKET_DESC

	feedback := root.ProtocolInstance().(*medco.PrivateCountProtocol).FeedbackChannel

	go root.StartProtocol()

	timeout := network.WaitRetry * time.Duration(network.MaxRetry*nNodes*2) * time.Millisecond

	select {
	case encryptedResult := <- feedback:
		result := medco.DecryptIntVector(suite, clientSecret, encryptedResult)
		dbg.Lvl1("RESULT:")
		dbg.Lvl1("Done with result =",result, "(target", targetCounts,")" )
		if !reflect.DeepEqual(targetCounts, result) {
			t.Fatal("Bad result.")
		}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}
