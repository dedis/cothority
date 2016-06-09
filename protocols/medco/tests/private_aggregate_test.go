package medco_test

import (
	"testing"
	//"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/lib/network"
	//"github.com/dedis/crypto/abstract"
	//"fmt"
	"time"
	_"reflect"
	//"github.com/dedis/cothority/services/medco/structs"
)

func TestPrivateAggregate5Nodes(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	host,entityList, tree := local.GenTree(5, false, true, true)
	defer local.CloseAll()
	
	
	p,err := local.CreateProtocol(tree, "PrivateAggregate")
	
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := p.(*medco.PrivateAggregateProtocol)
	
	suite := host[0].Suite()
	aggregateKey := entityList.Aggregate

	_=suite
	_=aggregateKey
	
	dbg.LLvl1("dataref")
	//DataReference creation
	testCipherVect := make(medco_structs.CipherVector, 4)
	for i, val := range []int64{1,2,3,6} {
		testCipherVect[i] = *medco_structs.EncryptInt(suite, aggregateKey, val)
	}
	
	var det1 medco_structs.DeterministCipherText
	var det2 medco_structs.DeterministCipherText
	det1.C = suite.Point().Base()
	det2.C = suite.Point().Add(det1.C, det1.C)
	groupAttr := medco_structs.GroupingAttributes{det1,det1}	
	groupAttr2 := medco_structs.GroupingAttributes{det2,det2}
	aggrAttr := testCipherVect
	var mpMessage map[medco_structs.GroupingAttributes]medco_structs.CipherVector
	mpMessage[groupAttr] = aggrAttr
	mpMessage[groupAttr2] = aggrAttr
	protocol.DataReference = &mpMessage
	
	
	dbg.LLvl1("Start PROTOCOL")
	go protocol.StartProtocol()
	timeout := network.WaitRetry * time.Duration(network.MaxRetry*5*2) * time.Millisecond
	
	feedback := protocol.FeedbackChannel

	select {
	case encryptedResult := <- feedback:
		dbg.Lvl1(local.Nodes)
		dbg.Lvl1("Recieved results", encryptedResult)
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

