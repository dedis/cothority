package medco_test

import (
	"testing"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/lib/network"
	"time"
	_"reflect"
	."github.com/dedis/cothority/services/medco/structs"
	"github.com/dedis/crypto/random"
)

func TestPrivateAggregate5Nodes(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 1)
	local := sda.NewLocalTest()
	_,/*entityList*/_, tree := local.GenTree(5, false, true, true)
	defer local.CloseAll()

	p,err := local.CreateProtocol(tree, "PrivateAggregate")
	//tni, _ := local.NewTreeNodeInstance(tree.Root, "PrivateAggregate")
	//p,err := medco.NewPrivateAggregate(tni)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := p.(*medco.PrivateAggregateProtocol)
	
	suite := network.Suite
	//aggregateKey := entityList.Aggregate
	// Generate client key
	clientPrivate := suite.Secret().Pick(random.Stream)
	clientPublic := suite.Point().Mul(suite.Point().Base(), clientPrivate)

	// Generate test data
	testGAMap := make(map[GroupingKey]GroupingAttributes)
	grpattr1 := DeterministCipherText{suite.Point().Base()}
	grpattr2 := DeterministCipherText{suite.Point().Null()}
	groupingAttrA,_ := DeterministicCipherVectorToGroupingAttributes(DeterministCipherVector{grpattr1, grpattr1})
	groupingAttrB,_ := DeterministicCipherVectorToGroupingAttributes(DeterministCipherVector{grpattr2, grpattr2})
	testGAMap[groupingAttrA.Key()] = groupingAttrA
	testGAMap[groupingAttrB.Key()] = groupingAttrB
	testCVMap := make(map[GroupingKey]CipherVector)
	testCVMap[groupingAttrA.Key()] = *EncryptIntArray(suite, clientPublic, []int64{1,2,3,4,5})
	testCVMap[groupingAttrB.Key()] = *EncryptIntArray(suite, clientPublic, []int64{6,7,8,9,10})

	protocol.Groups = &testGAMap
	protocol.GroupedData = &testCVMap

	go protocol.Dispatch()
	go protocol.StartProtocol()
	timeout := network.WaitRetry * time.Duration(network.MaxRetry*5*2) * time.Millisecond
	
	feedback := protocol.FeedbackChannel

	select {
	case encryptedResult := <- feedback:
		dbg.Lvl1("Recieved results:")
		for k := range encryptedResult.GroupedData {
			dbg.Lvl1(k, DecryptIntVector(suite, clientPrivate, encryptedResult.GroupedData[k]))
		}

	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

