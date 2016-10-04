package sda

import (
	"errors"
	"fmt"
	"testing"

	"reflect"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/config"
)

var testServiceID ServiceID
var testMsgID network.PacketTypeID

func init() {
	RegisterNewService("testService", newTestService)
	testServiceID = ServiceFactory.ServiceID("testService")
	testMsgID = network.RegisterPacketType(&testMsg{})
}

func TestProcessor_AddMessage(t *testing.T) {
	h1 := NewLocalConode(2000)
	defer h1.Close()
	p := NewServiceProcessor(&Context{conode: h1})
	log.ErrFatal(p.RegisterMessage(procMsg))
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
		log.Lvl2("Checking function", reflect.TypeOf(f).String())
		err := p.RegisterMessage(f)
		if err == nil {
			t.Fatalf("Shouldn't accept function %+s", reflect.TypeOf(f).String())
		}
	}
}

func TestProcessor_RegisterMessages(t *testing.T) {
	p := NewServiceProcessor(&Context{})
	log.ErrFatal(p.RegisterMessages(procMsg, procMsg2))
	err := p.RegisterMessages(procMsg, procMsgWrong1)
	if err == nil {
		t.Fatal("Registered wrong message and didn't get an error")
	}
}

func TestProcessor_GetReply(t *testing.T) {
	h1 := NewLocalConode(2000)
	defer h1.Close()
	p := NewServiceProcessor(&Context{conode: h1})
	log.ErrFatal(p.RegisterMessage(procMsg))

	pair := config.NewKeyPair(network.Suite)
	e := network.NewServerIdentity(pair.Public, "")

	rep := p.GetReply(e, testMsgID, testMsg{11})
	val, ok := rep.(*testMsg)
	if !ok {
		t.Fatalf("Couldn't cast reply to testMsg: %+v", rep)
	}
	if val.I != 11 {
		t.Fatal("Value got lost - should be 11")
	}

	rep = p.GetReply(e, testMsgID, testMsg{42})
	errMsg, ok := rep.(*network.StatusRet)
	if !ok {
		t.Fatal("42 should return an error")
	}
	if errMsg.Status == "" {
		t.Fatal("The error should be non-empty")
	}
}

func TestProcessor_ProcessClientRequest(t *testing.T) {
	local := NewLocalTest()

	// generate 5 hosts,
	h := local.GenConodes(1)[0]
	defer local.CloseAll()

	s := local.Services[h.ServerIdentity.ID]
	ts := s[testServiceID]
	client := local.NewClient("testService")

	resp, err := client.Send(h.ServerIdentity, &testMsg{12})
	if err != nil {
		t.Fatal("error sending /receiving with client")
	}
	msg := ts.(*testService).Msg
	if msg == nil {
		t.Fatal("Msg should not be nil")
	}
	tm, ok := resp.Msg.(testMsg)
	if !ok {
		t.Fatalf("Couldn't cast to *testMsg - %+v", tm)
	}
	if tm.I != 12 {
		t.Fatal("Didn't send 12")
	}

}

func mkClientRequest(msg network.Body) []byte {
	b, err := network.MarshalRegisteredType(msg)
	log.ErrFatal(err)
	return b
}

type testMsg struct {
	I int
}

func procMsg(si *network.ServerIdentity, msg *testMsg) (network.Body, error) {
	// Return an error for testing
	if msg.I == 42 {
		return nil, fmt.Errorf("ServiceProcessor received %d (actual) vs 42 (expected)", msg.I)
	}
	return msg, nil
}

func procMsg2(si *network.ServerIdentity, msg *testMsg) (network.Body, error) {
	// Return an error for testing
	if msg.I != 42 {
		return nil, errors.New("Please give meaning of life.")
	}
	return msg, nil
}

func procMsgWrong1(msg *testMsg) (network.Body, error) {
	return msg, nil
}

func procMsgWrong2(si *network.ServerIdentity) (network.Body, error) {
	return nil, nil
}

func procMsgWrong3(si *network.ServerIdentity, msg testMsg) (network.Body, error) {
	return msg, nil
}

func procMsgWrong4(si *network.ServerIdentity, msg *testMsg) error {
	return nil
}

func procMsgWrong5(si *network.ServerIdentity, msg *testMsg) (error, network.Body) {
	return nil, msg
}

func procMsgWrong6(si *network.ServerIdentity, msg *int) (network.Body, error) {
	return msg, nil
}

type testService struct {
	*ServiceProcessor
	Msg interface{}
}

func newTestService(c *Context, path string) Service {
	ts := &testService{
		ServiceProcessor: NewServiceProcessor(c),
	}
	ts.RegisterMessage(ts.ProcessMsg)
	return ts
}

func (ts *testService) NewProtocol(tn *TreeNodeInstance, conf *GenericConfig) (ProtocolInstance, error) {
	return nil, nil
}

func (ts *testService) ProcessMsg(si *network.ServerIdentity, msg *testMsg) (network.Body, error) {
	ts.Msg = msg
	return msg, nil
}

func returnMsg(si *network.ServerIdentity, msg network.Body) (network.Body, error) {
	return msg, nil
}
