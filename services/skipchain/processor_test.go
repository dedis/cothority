package skipchain

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"reflect"
	"github.com/dedis/crypto/config"
)

func TestProcessor(t *testing.T) {
	p := &Processor{}
	p.AddMessage(returnMsg)
	msg := &testMsg{10}
	b, err := network.MarshalRegisteredType(msg)
	dbg.ErrFatal(err)
	cr := &sda.ClientRequest{
		Type: network.TypeFromData(msg),
		Data: b,
	}
	reply := p.GetReply(nil, cr)
	tm, ok := reply.(testMsg)
	if !ok {
		t.Fatal("Couldn't cast to *testMsg")
	}
	if tm.I != 10 {
		t.Fatal("Lost value in between")
	}
}

func TestProcessor_AddMessage(t *testing.T) {
	p := NewProcessor()
	dbg.ErrFatal(p.AddMessage(procMsg))
	if len(p.functions) != 1{
		t.Fatal("Should have registered one function")
	}
	mt := network.TypeFromData(&testMsg{})
	if mt == network.ErrorType{
		t.Fatal("Didn't register message-type correctly")
	}
	var wrongFunctions = []interface{}{
		procMsgWrong1,
		procMsgWrong2,
		procMsgWrong3,
		procMsgWrong4,
		procMsgWrong5,
		procMsgWrong6,
	}
	for _, f := range wrongFunctions{
		dbg.Lvl2("Checking function %+v", reflect.TypeOf(f).String())
		err := p.AddMessage(f)
		if err == nil{
			t.Fatalf("Shouldn't accept function %+v", reflect.TypeOf(f).String())
		}
	}
}

func TestProcessor_GetReply(t *testing.T) {
	p := NewProcessor()
	dbg.ErrFatal(p.AddMessage(procMsg))

	pair := config.NewKeyPair(network.Suite)
	e := network.NewEntity(pair.Public, "")
	b, err := network.MarshalRegisteredType(&testMsg{11})
	dbg.ErrFatal(err)
	request := &sda.ClientRequest{
		Type: network.TypeFromData(&testMsg{}),
		Data: b,
	}

	rep := p.GetReply(e, request)
	val, ok := rep.(*testMsg)
	if !ok{
		t.Fatalf("Couldn't cast reply to testMsg: %+v", rep)
	}
	if val.I != 11{
		t.Fatal("Value got lost - should be 11")
	}
}

func procMsg(e *network.Entity, msg *testMsg) (network.ProtocolMessage, error) {
	return msg, nil
}

func procMsgWrong1(msg *testMsg) (network.ProtocolMessage, error) {
	return msg, nil
}

func procMsgWrong2(e *network.Entity) (network.ProtocolMessage, error) {
	return nil, nil
}

func procMsgWrong3(e *network.Entity, msg testMsg) (network.ProtocolMessage, error) {
	return msg, nil
}

func procMsgWrong4(e *network.Entity, msg *testMsg) (error) {
	return nil
}

func procMsgWrong5(e *network.Entity, msg *testMsg) (error, network.ProtocolMessage) {
	return nil, msg
}

func procMsgWrong6(e *network.Entity, msg *int) (network.ProtocolMessage, error) {
	return msg, nil
}


type testService struct {
	*Processor
}

func newTestService(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) sda.Service {
	return &testService{
		Processor: NewProcessor(),
	}
}

func (ts *testService) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	return nil, nil
}

func (ts *testService) ProcessMsg(e *network.Entity, msg *testMsg) (network.ProtocolMessage, error) {
	return msg, nil
}

func returnMsg(e *network.Entity, msg network.ProtocolMessage) (network.ProtocolMessage, error) {
	return msg, nil
}
