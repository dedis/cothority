package sda

import (
	"testing"
	"time"

	"reflect"

	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/assert"
)

type basicProcessor struct {
	msgChan chan network.Packet
}

func (bp *basicProcessor) Process(msg *network.Packet) {
	bp.msgChan <- *msg
}

type basicMessage struct {
	Value int
}

var basicMessageType network.MessageTypeID

func TestBlockingDispatcher(t *testing.T) {
	defer log.AfterTest(t)

	dispatcher := NewBlockingDispatcher()
	processor := &basicProcessor{make(chan network.Packet, 1)}

	dispatcher.RegisterProcessor(processor, basicMessageType)
	dispatcher.Dispatch(&network.Packet{
		Msg:     basicMessage{10},
		MsgType: basicMessageType})

	select {
	case m := <-processor.msgChan:
		msg, ok := m.Msg.(basicMessage)
		assert.True(t, ok)
		assert.Equal(t, msg.Value, 10)
	default:
		t.Error("No message received")
	}
}

func TestProcessorHost(t *testing.T) {
	defer log.AfterTest(t)
	h1 := NewTestHost(2000)
	defer h1.Close()

	proc := &basicProcessor{make(chan network.Packet, 1)}
	h1.RegisterProcessor(proc, basicMessageType)
	assert.Nil(t, h1.Dispatch(&network.Packet{
		Msg:     basicMessage{10},
		MsgType: basicMessageType}))

	select {
	case m := <-proc.msgChan:
		basic, ok := m.Msg.(basicMessage)
		assert.True(t, ok)
		assert.Equal(t, basic.Value, 10)
	case <-time.After(100 * time.Millisecond):
		t.Error("No message received on channel")
	}
}

var testServiceID ServiceID
var testMsgID network.MessageTypeID

func init() {
	basicMessageType = network.RegisterMessageType(&basicMessage{})
	RegisterNewService("testService", newTestService)
	testServiceID = ServiceFactory.ServiceID("testService")
	testMsgID = network.RegisterMessageType(&testMsg{})
}

func TestProcessor_AddMessage(t *testing.T) {
	h1 := NewTestHost(2000)
	defer h1.Close()
	p := NewServiceProcessor(&Context{host: h1})
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

func TestProcessor_GetReply(t *testing.T) {
	h1 := NewTestHost(2000)
	defer h1.Close()
	p := NewServiceProcessor(&Context{host: h1})
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
	h := local.GenTestHosts(1, false, false)[0]
	defer local.CloseAll()

	s := local.Services[h.ServerIdentity.ID]
	ts := s[testServiceID]
	cr := &ClientRequest{Data: mkClientRequest(&testMsg{12})}
	ts.ProcessClientRequest(h.ServerIdentity, cr)
	msg := ts.(*testService).Msg
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

func mkClientRequest(msg network.Body) []byte {
	b, err := network.MarshalRegisteredType(msg)
	log.ErrFatal(err)
	return b
}

type testMsg struct {
	I int
}

func procMsg(e *network.ServerIdentity, msg *testMsg) (network.Body, error) {
	// Return an error for testing
	if msg.I == 42 {
		return nil, errors.New("6 * 9 != 42")
	}
	return msg, nil
}

func procMsgWrong1(msg *testMsg) (network.Body, error) {
	return msg, nil
}

func procMsgWrong2(e *network.ServerIdentity) (network.Body, error) {
	return nil, nil
}

func procMsgWrong3(e *network.ServerIdentity, msg testMsg) (network.Body, error) {
	return msg, nil
}

func procMsgWrong4(e *network.ServerIdentity, msg *testMsg) error {
	return nil
}

func procMsgWrong5(e *network.ServerIdentity, msg *testMsg) (error, network.Body) {
	return nil, msg
}

func procMsgWrong6(e *network.ServerIdentity, msg *int) (network.Body, error) {
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

func (ts *testService) ProcessMsg(e *network.ServerIdentity, msg *testMsg) (network.Body, error) {
	ts.Msg = msg
	return msg, nil
}

func returnMsg(e *network.ServerIdentity, msg network.Body) (network.Body, error) {
	return msg, nil
}
