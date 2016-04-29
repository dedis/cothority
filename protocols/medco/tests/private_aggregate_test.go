package medco_test

import (
	"testing"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/lib/network"
	"time"
	_"reflect"
)

func TestPrivateAggregate5Nodes(t *testing.T) {
	defer dbg.AfterTest(t)
	local := sda.NewLocalTest()
	dbg.TestOutput(testing.Verbose(), 1)
	host,entityList, tree := local.GenTree(5, false, true, true)
	defer local.CloseAll()

	rootInstance,_ := local.CreateProtocol("PrivateAggregate", tree)
	protocol := rootInstance.(*medco.PrivateAggregateProtocol)

	suite := host[0].Suite()
	aggregateKey := entityList.Aggregate

	_=suite
	_=aggregateKey


	feedback := protocol.FeedbackChannel

	go protocol.StartProtocol()

	timeout := network.WaitRetry * time.Duration(network.MaxRetry*5*2) * time.Millisecond


	select {
	case encryptedResult := <- feedback:
		dbg.Lvl1(local.Nodes)
		dbg.Lvl1("Recieved results", encryptedResult)
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

