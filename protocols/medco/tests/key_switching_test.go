package medco_test

import (
	"testing"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/lib/network"
	"time"
	_"reflect"
	"github.com/dedis/crypto/random"
	"reflect"
)

func TestKeySwitching5Nodes(t *testing.T) {
	defer dbg.AfterTest(t)
	local := sda.NewLocalTest()
	dbg.TestOutput(testing.Verbose(), 1)
	host,entityList, tree := local.GenTree(5, false, true, true)
	defer local.CloseAll()

	rootInstance,_ := local.CreateProtocol("KeySwitching", tree)
	protocol := rootInstance.(*medco.KeySwitchingProtocol)

	suite := host[0].Suite()
	aggregateKey := entityList.Aggregate

	// Encrypt test data with group key
	testCipherVect := make(medco.CipherVector, 4)
	expRes := []int64{1,2,3,6}
	for i, p := range expRes {
		testCipherVect[i] = *medco.EncryptInt(suite, aggregateKey, p)
	}

	// Generate client key
	clientPrivate := suite.Secret().Pick(random.Stream)
	clientPublic := suite.Point().Mul(suite.Point().Base(), clientPrivate)

	protocol.TargetOfSwitch = &testCipherVect
	protocol.TargetPublicKey = &clientPublic
	feedback := protocol.FeedbackChannel

	go protocol.StartProtocol()

	timeout := network.WaitRetry * time.Duration(network.MaxRetry*5*2) * time.Millisecond

	select {
	case encryptedResult := <- feedback:
		res := medco.DecryptIntVector(suite, clientPrivate,encryptedResult)
		dbg.Lvl1("Recieved results", res)
		if !reflect.DeepEqual(res,expRes ){
			t.Fatal("Wrong results, expected", expRes, "but got", res)
		}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

