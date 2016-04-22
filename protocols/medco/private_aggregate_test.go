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
	nNodes := 5
	dbg.TestOutput(testing.Verbose(), 1)
	host,entityList, tree := local.GenTree(nNodes, false, true, true)
	defer local.CloseAll()

	root,_ := local.CreateNewNodeName("PrivateAggregate", tree)

	suite := host[0].Suite()
	aggregateKey := entityList.Aggregate

	_=suite
	_=aggregateKey


	ref := medco.DataRef(0)
	root.ProtocolInstance().(*medco.PrivateAggregateProtocol).DataReference = &ref
	feedback := root.ProtocolInstance().(*medco.PrivateAggregateProtocol).FeedbackChannel

	go root.StartProtocol()

	timeout := network.WaitRetry * time.Duration(network.MaxRetry*nNodes*2) * time.Millisecond

	dbg.Lvl1(local.Nodes)

	select {
	case encryptedResult := <- feedback:
		dbg.Lvl1("Recieved results", encryptedResult)
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

