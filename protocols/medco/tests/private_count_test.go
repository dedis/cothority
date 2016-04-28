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
const NUM_BUCKET = 2
var BUCKET_DESC = []int64{50}
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
	dbg.TestOutput(testing.Verbose(), 1)
	host,entityList, tree := local.GenTree(5, true, true, true)
	defer local.CloseAll()


	rootInstance,_ := local.CreateProtocol("PrivateCount", tree)
	protocol := rootInstance.(*medco.PrivateCountProtocol)

	dbg.Lvl1(local.Nodes) // == []

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


	protocol.EncryptedData = &EncryptedData
	protocol.ClientPublicKey = &clientPublic
	protocol.ClientQuery = clientQuery
	protocol.BucketDesc = &BUCKET_DESC

	feedback := protocol.FeedbackChannel

	go protocol.StartProtocol()

	for i:=0; i < 10; i++ {
		dbg.Lvl1(local.Nodes)// == []
		<- time.After(1*time.Second)
	}


	timeout := network.WaitRetry * time.Duration(network.MaxRetry*5*2) * time.Millisecond

	select {
	case encryptedResult := <- feedback:

		dbg.Lvl1(local.Nodes)// == []


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
