package medco_test

import (
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/sda"
	. "github.com/dedis/cothority/services/medco/libmedco"
	"github.com/dedis/crypto/random"
	"testing"
	"time"
)

func TestDeterministicSwitching(t *testing.T) {
	defer log.AfterTest(t)
	local := sda.NewLocalTest()
	log.TestOutput(testing.Verbose(), 1)
	_, entityList, tree := local.GenTree(5, false, true, true)
	sda.ProtocolRegisterName("DeterministicSwitchingTest", NewDeterministicSwitchingTest)
	defer local.CloseAll()

	rootInstance, _ := local.CreateProtocol(tree, "DeterministicSwitchingTest")
	protocol := rootInstance.(*medco.DeterministicSwitchingProtocol)

	aggregateKey := entityList.Aggregate

	testCipherVect := make(CipherVector, 4)
	expRes := []int64{1, 2, 3, 6}
	for i, p := range expRes {
		testCipherVect[i] = *EncryptInt(aggregateKey, p)
	}

	testCipherVect1 := make(CipherVector, 4)
	expRes1 := []int64{1, 2, 3, 6}
	for i, p := range expRes1 {
		testCipherVect1[i] = *EncryptInt(aggregateKey, p)
	}

	testCipherVect2 := make(CipherVector, 4)
	expRes2 := []int64{1, 2, 2, 2}
	for i, p := range expRes2 {
		testCipherVect2[i] = *EncryptInt(aggregateKey, p)
	}

	var mapi map[TempID]CipherVector
	mapi = make(map[TempID]CipherVector)
	mapi[TempID(1)] = testCipherVect
	mapi[TempID(2)] = testCipherVect1
	mapi[TempID(3)] = testCipherVect2

	protocol.TargetOfSwitch = &mapi

	feedback := protocol.FeedbackChannel
	go protocol.StartProtocol()

	timeout := network.WaitRetry * time.Duration(network.MaxRetry*5*2) * time.Millisecond

	select {
	case encryptedResult := <-feedback:
		resultDet1 := encryptedResult[TempID(1)]
		log.Lvl1("Recieved results", resultDet1)
		resultDet2 := encryptedResult[TempID(2)]
		log.Lvl1("Recieved results", resultDet2)
		resultDet3 := encryptedResult[TempID(3)]
		log.Lvl1("Recieved results", resultDet3)
		if !resultDet1.Equal(&resultDet2) {
			t.Fatal("Wrong results, expected same values but got ", resultDet1, " & ", resultDet2)
		}
		if resultDet1.Equal(&resultDet3) {
			t.Fatal("Wrong results, expected different values but got ", resultDet1, " & ", resultDet3)
		}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

// NewDeterministicSwitchingTest is a special purpose protocol constructor specific to tests.
func NewDeterministicSwitchingTest(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	pi, err := medco.NewDeterministSwitchingProtocol(tni)
	protocol := pi.(*medco.DeterministicSwitchingProtocol)

	clientPrivate := network.Suite.Scalar().Pick(random.Stream)
	protocol.SurveyPHKey = &clientPrivate

	return protocol, err
}
