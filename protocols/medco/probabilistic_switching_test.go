package medco_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/medco/libmedco"
	"github.com/dedis/crypto/random"
)

// TestProbabilisticSwitching tests probabilistic switching protocol
func TestProbabilisticSwitching(t *testing.T) {
	local := sda.NewLocalTest()
	_, entityList, tree := local.GenTree(5, true)
	sda.ProtocolRegisterName("ProbabilisticSwitchingTest", NewProbabilisticSwitchingTest)

	defer local.CloseAll()

	var clientPrivate = network.Suite.Scalar().Pick(random.Stream)
	var clientPublic = network.Suite.Point().Mul(network.Suite.Point().Base(), clientPrivate)

	rootInstance, _ := local.CreateProtocol("ProbabilisticSwitchingTest", tree)
	protocol := rootInstance.(*medco.ProbabilisticSwitchingProtocol)

	aggregateKey := entityList.Aggregate

	expRes := []int64{1, 1}
	point := network.Suite.Scalar().SetInt64(1)
	multPoint := network.Suite.Point().Mul(network.Suite.Point().Base(), point)
	multPoint.Add(multPoint, aggregateKey)
	det := libmedco.DeterministCipherText{Point: multPoint}

	var mapi map[libmedco.TempID]libmedco.DeterministCipherVector
	mapi = make(map[libmedco.TempID]libmedco.DeterministCipherVector)
	mapi[libmedco.TempID(1)] = libmedco.DeterministCipherVector{det, det}
	mapi[libmedco.TempID(1)] = libmedco.DeterministCipherVector{det, det}
	protocol.TargetOfSwitch = &mapi
	protocol.TargetPublicKey = &clientPublic

	feedback := protocol.FeedbackChannel

	go protocol.StartProtocol()

	timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*5*2) * time.Millisecond

	select {
	case encryptedResult := <-feedback:
		val1 := encryptedResult[libmedco.TempID(1)]
		cv1 := libmedco.DecryptIntVector(clientPrivate, &val1)
		val2 := encryptedResult[libmedco.TempID(1)]
		cv2 := libmedco.DecryptIntVector(clientPrivate, &val2)
		if !reflect.DeepEqual(cv1, expRes) {
			t.Fatal("Wrong results, expected ", expRes, " and got ", cv1)
		}
		if !reflect.DeepEqual(cv2, expRes) {
			t.Fatal("Wrong results, expected ", expRes, " and got ", cv2)
		}

	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

// NewProbabilisticSwitchingTest is a test specific constructor that injects data in the protocol instance.
func NewProbabilisticSwitchingTest(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	pi, err := medco.NewProbabilisticSwitchingProtocol(tni)
	protocol := pi.(*medco.ProbabilisticSwitchingProtocol)
	priv := protocol.Private()
	protocol.SurveyPHKey = &priv

	return protocol, err
}
