package medco_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/random"
	"github.com/stretchr/testify/assert"
)

var suite = network.Suite
var clientPrivate = suite.Scalar().Pick(random.Stream)
var clientPublic = suite.Point().Mul(suite.Point().Base(), clientPrivate)
var grpattr1 = DeterministCipherText{suite.Point().Base()}
var grpattr2 = DeterministCipherText{suite.Point().Null()}
var groupingAttrA = GroupingAttributes{grpattr1, grpattr1}
var groupingAttrB = GroupingAttributes{grpattr2, grpattr2}
var groupingAttrC = GroupingAttributes{grpattr1, grpattr2}

//TestPrivateAggregate tests private aggregate protocol
func TestPrivateAggregate(t *testing.T) {
	defer log.AfterTest(t)
	log.TestOutput(testing.Verbose(), 1)
	local := sda.NewLocalTest()
	_ /*entityList*/, _, tree := local.GenTree(10, false, true, true)
	sda.ProtocolRegisterName("PrivateAggregateTest", NewPrivateAggregateTest)
	defer local.CloseAll()

	p, err := local.CreateProtocol(tree, "PrivateAggregateTest")
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := p.(*medco.PrivateAggregateProtocol)

	//run protocol
	go protocol.StartProtocol()
	timeout := network.WaitRetry * time.Duration(network.MaxRetry*5*2) * time.Millisecond

	feedback := protocol.FeedbackChannel

	//verify results
	expectedGroups := map[GroupingKey]GroupingAttributes{groupingAttrA.Key(): groupingAttrA,
		groupingAttrB.Key(): groupingAttrB,
		groupingAttrC.Key(): groupingAttrC}

	expectedResults := map[GroupingKey][]int64{groupingAttrA.Key(): {3, 5, 7, 9, 11},
		groupingAttrB.Key(): {1, 2, 3, 4, 5},
		groupingAttrC.Key(): {1, 1, 1, 1, 1}}

	select {
	case encryptedResult := <-feedback:
		log.Lvl1("Recieved results:")
		resultData := make(map[GroupingKey][]int64)
		for k, v := range encryptedResult.GroupedData {
			resultData[k] = DecryptIntVector(clientPrivate, &v)
			log.Lvl1(k, resultData[k])
		}
		for k, v1 := range expectedGroups {
			if v2, ok := encryptedResult.Groups[k]; ok {
				assert.True(t, ok)
				assert.True(t, v1.Equal(&v2))
				delete(encryptedResult.Groups, k)
			}
		}
		assert.Empty(t, encryptedResult.Groups)
		assert.Equal(t, expectedResults, resultData)
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

// NewPrivateAggregateTest is a test specific protocol instance constructor that injects test data.
func NewPrivateAggregateTest(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	pi, err := medco.NewPrivateAggregate(tni)
	protocol := pi.(*medco.PrivateAggregateProtocol)

	testGAMap := make(map[GroupingKey]GroupingAttributes)
	testCVMap := make(map[GroupingKey]CipherVector)

	switch tni.Index() {
	case 0:

		testGAMap[groupingAttrA.Key()] = groupingAttrA
		testCVMap[groupingAttrA.Key()] = *EncryptIntVector(clientPublic, []int64{1, 2, 3, 4, 5})
		testGAMap[groupingAttrB.Key()] = groupingAttrB
		testCVMap[groupingAttrB.Key()] = *EncryptIntVector(clientPublic, []int64{0, 0, 0, 0, 0})
	case 1:
		testGAMap[groupingAttrB.Key()] = groupingAttrB
		testCVMap[groupingAttrB.Key()] = *EncryptIntVector(clientPublic, []int64{1, 2, 3, 4, 5})
	case 2:
		testGAMap[groupingAttrA.Key()] = groupingAttrA
		testCVMap[groupingAttrA.Key()] = *EncryptIntVector(clientPublic, []int64{1, 1, 1, 1, 1})
	case 9:
		testGAMap[groupingAttrC.Key()] = groupingAttrC
		testCVMap[groupingAttrC.Key()] = *EncryptIntVector(clientPublic, []int64{1, 0, 1, 0, 1})
		testGAMap[groupingAttrA.Key()] = groupingAttrA
		testCVMap[groupingAttrA.Key()] = *EncryptIntVector(clientPublic, []int64{1, 2, 3, 4, 5})
	case 5:
		testGAMap[groupingAttrC.Key()] = groupingAttrC
		testCVMap[groupingAttrC.Key()] = *EncryptIntVector(clientPublic, []int64{0, 1, 0, 1, 0})
	default:
	}
	protocol.Groups = &testGAMap
	protocol.GroupedData = &testCVMap

	return protocol, err
}
