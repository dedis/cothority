package medco_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/medco"
	. "github.com/dedis/cothority/services/medco/structs"
	"github.com/dedis/crypto/random"
	"reflect"
	_ "reflect"
	"testing"
	"time"
)

func TestDeterministicSwitching5Nodes(t *testing.T) {
	defer dbg.AfterTest(t)
	local := sda.NewLocalTest()
	dbg.TestOutput(testing.Verbose(), 1)
	host, entityList, tree := local.GenTree(5, false, true, true)
	defer local.CloseAll()

	rootInstance, _ := local.CreateProtocol(tree, "DeterministicSwitching")
	protocol := rootInstance.(*medco.DeterministicSwitchingProtocol)

	suite := host[0].Suite()
	aggregateKey := entityList.Aggregate

	// Encrypt test data with group key
	testCipherVect := make(CipherVector, 4)
	expRes := []int64{1, 2, 3, 6}
	for i, p := range expRes {
		testCipherVect[i] = *EncryptInt(suite, aggregateKey, p)
	}

	testCipherVect1 := make(CipherVector, 4)
	expRes1 := []int64{1, 2, 3, 6}
	for i, p := range expRes1 {
		testCipherVect1[i] = *EncryptInt(suite, aggregateKey, p)
	}

	var mapi map[TempID]CipherVector
	mapi = make(map[TempID]CipherVector)
	mapi[TempID(1)] = testCipherVect
	mapi[TempID(2)] = testCipherVect1
	//mapi[TempID(3)] = testCipherVect1

	// Generate client key
	clientPrivate := suite.Secret().Pick(random.Stream)
	//clientPublic := suite.Point().Mul(suite.Point().Base(), clientPrivate)

	protocol.TargetOfSwitch = &mapi
	protocol.SurveyPHKey = &clientPrivate

	dbg.LLvl1(protocol.SurveyPHKey)
	feedback := protocol.FeedbackChannel

	go protocol.StartProtocol()

	timeout := network.WaitRetry * time.Duration(network.MaxRetry*5*2) * time.Millisecond

	select {
	case encryptedResult := <-feedback:
		res := encryptedResult[TempID(1)]
		dbg.Lvl1("Recieved results", res)
		res1 := encryptedResult[TempID(2)]
		dbg.Lvl1("Recieved results", res1)
		if !reflect.DeepEqual(res, res1) {
			t.Fatal("Wrong results, expected", expRes, "but got", res)
		}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}
