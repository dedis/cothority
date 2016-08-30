package medco_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/medco/libmedco"
	"github.com/dedis/crypto/random"
)

func TestKeySwitching(t *testing.T) {
	local := sda.NewLocalTest()
	_, entityList, tree := local.GenTree(5, false, true, true)

	defer local.CloseAll()

	rootInstance, err := local.CreateProtocol("KeySwitching", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := rootInstance.(*medco.KeySwitchingProtocol)

	suite := network.Suite
	aggregateKey := entityList.Aggregate

	expRes := []int64{1, 2, 3, 6}
	testCipherVect := *libmedco.EncryptIntVector(aggregateKey, expRes)

	expRes1 := []int64{7, 8, 9, 7}
	testCipherVect1 := *libmedco.EncryptIntVector(aggregateKey, expRes1)

	var mapi map[libmedco.TempID]libmedco.CipherVector
	mapi = make(map[libmedco.TempID]libmedco.CipherVector)
	mapi[libmedco.TempID(1)] = testCipherVect
	mapi[libmedco.TempID(2)] = testCipherVect1

	clientPrivate := suite.Scalar().Pick(random.Stream)
	clientPublic := suite.Point().Mul(suite.Point().Base(), clientPrivate)

	protocol.TargetOfSwitch = &mapi

	protocol.TargetPublicKey = &clientPublic
	feedback := protocol.FeedbackChannel

	go protocol.StartProtocol()

	timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*5*2) * time.Millisecond

	select {
	case encryptedResult := <-feedback:
		cv1 := encryptedResult[libmedco.TempID(1)]
		res := libmedco.DecryptIntVector(clientPrivate, &cv1)
		log.Lvl1("Recieved results", res)
		cv2 := encryptedResult[libmedco.TempID(2)]
		res1 := libmedco.DecryptIntVector(clientPrivate, &cv2)
		log.Lvl1("Recieved results", res1)
		if !reflect.DeepEqual(res, expRes) {
			t.Fatal("Wrong results, expected", expRes, "but got", res)
		} else {
			t.Log("Good results")
		}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}
