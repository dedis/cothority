package medco_test

import (
	"testing"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/lib/network"
	"time"
	"github.com/dedis/crypto/random"
	."github.com/dedis/cothority/services/medco/structs"
)

func TestKeySwitching5Nodes(t *testing.T) {
	defer dbg.AfterTest(t)
	local := sda.NewLocalTest()
	dbg.TestOutput(testing.Verbose(), 1)
	_,entityList, tree := local.GenTree(5, false, true, true)
	defer local.CloseAll()

	rootInstance,err := local.CreateProtocol(tree, "KeySwitching")
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := rootInstance.(*medco.KeySwitchingProtocol)

	suite := network.Suite
	aggregateKey := entityList.Aggregate
	// Generate client key
	clientPrivate := suite.Secret().Pick(random.Stream)
	clientPublic := suite.Point().Mul(suite.Point().Base(), clientPrivate)

	// Encrypt test data with group key
	testMap := make(map[TempID]CipherVector)
	for  i := int64(0); i < 3; i++ {
		testMap[TempID(i)] = *EncryptIntArray(suite, aggregateKey, []int64{i+1,i+2,i+3,i+4,i+5})
	}

	protocol.TargetOfSwitch = &testMap
	protocol.TargetPublicKey = &clientPublic
	feedback := protocol.FeedbackChannel

	go protocol.StartProtocol()

	timeout := network.WaitRetry * time.Duration(network.MaxRetry*5*2) * time.Millisecond

	select {
	case encryptedResult := <- feedback:
		dbg.Lvl1("Recieved results:")
		for k := range encryptedResult {
			dbg.Lvl1(k, DecryptIntVector(suite, clientPrivate, encryptedResult[k]))
		}

		//if !reflect.DeepEqual(res,expRes ){
		//	t.Fatal("Wrong results, expected", expRes, "but got", res)
		//}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

