package sda

import (
	"testing"

	"reflect"

	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/config"
)

var testServiceID ServiceID

func init() {
	RegisterNewService("testService", newTestService)
	testServiceID = ServiceFactory.ServiceID("testService")
}

func TestProcessor_AddMessage(t *testing.T) {
	p := NewServiceProcessor(nil)
	dbg.ErrFatal(p.RegisterMessage(procMsg))
	if len(p.functions) != 1 {
		t.Fatal("Should have registered one function")
	}
	mt := network.TypeFromData(&testMsg{})
	if mt == network.ErrorType {
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
	for _, f := range wrongFunctions {
		dbg.Lvl2("Checking function", reflect.TypeOf(f).String())
		err := p.RegisterMessage(f)
		if err == nil {
			t.Fatalf("Shouldn't accept function %+s", reflect.TypeOf(f).String())
		}
	}
}

func TestProcessor_GetReply(t *testing.T) {
	p := NewServiceProcessor(nil)
	dbg.ErrFatal(p.RegisterMessage(procMsg))

	pair := config.NewKeyPair(network.Suite)
	e := network.NewEntity(pair.Public, "")

	rep := p.GetReply(e, mkClientRequest(&testMsg{11}))
	val, ok := rep.(*testMsg)
	if !ok {
		t.Fatalf("Couldn't cast reply to testMsg: %+v", rep)
	}
	if val.I != 11 {
		t.Fatal("Value got lost - should be 11")
	}

	rep = p.GetReply(e, mkClientRequest(&testMsg{42}))
	errMsg, ok := rep.(*StatusRet)
	if !ok {
		t.Fatal("42 should return an error")
	}
	if errMsg.Status == "" {
		t.Fatal("The error should be non-empty")
	}
}

func TestProcessor_ProcessClientRequest(t *testing.T) {
	local := NewLocalTest()

	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	h := local.GenLocalHosts(1, false, false)[0]
	defer local.CloseAll()

	s := local.Services[h.Entity.ID]
	ts := s[testServiceID]
	ts.ProcessClientRequest(h.Entity, mkClientRequest(&testMsg{12}))
	msg := ts.(*testService).Context.(*testContext).Msg
	if msg == nil {
		t.Fatal("Msg should not be nil")
	}
	tm, ok := msg.(*testMsg)
	if !ok {
		t.Fatalf("Couldn't cast to *testMsg - %+v", tm)
	}
	if tm.I != 12 {
		t.Fatal("Didn't send 12")
	}
}

func mkClientRequest(msg network.ProtocolMessage) *ClientRequest {
	b, err := network.MarshalRegisteredType(msg)
	dbg.ErrFatal(err)
	return &ClientRequest{
		Data: b,
	}
}

type testMsg struct {
	I int
}

func procMsg(e *network.Entity, msg *testMsg) (network.ProtocolMessage, error) {
	// Return an error for testing
	if msg.I == 42 {
		return nil, errors.New("6 * 9 != 42")
	}
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

func procMsgWrong4(e *network.Entity, msg *testMsg) error {
	return nil
}

func procMsgWrong5(e *network.Entity, msg *testMsg) (error, network.ProtocolMessage) {
	return nil, msg
}

func procMsgWrong6(e *network.Entity, msg *int) (network.ProtocolMessage, error) {
	return msg, nil
}

type testService struct {
	*ServiceProcessor
}

type testContext struct {
	Context
	Msg interface{}
}

func newTestService(c Context, path string) Service {
	ts := &testService{
		ServiceProcessor: NewServiceProcessor(&testContext{Context: c}),
	}
	ts.RegisterMessage(ts.ProcessMsg)
	return ts
}

func (ts *testService) NewProtocol(tn *TreeNodeInstance, conf *GenericConfig) (ProtocolInstance, error) {
	return nil, nil
}

func (ts *testService) ProcessMsg(e *network.Entity, msg *testMsg) (network.ProtocolMessage, error) {
	return msg, nil
}

func (ts *testContext) SendRaw(to *network.Entity, msg interface{}) error {
	ts.Msg = msg
	return nil
}

func returnMsg(e *network.Entity, msg network.ProtocolMessage) (network.ProtocolMessage, error) {
	return msg, nil
}
