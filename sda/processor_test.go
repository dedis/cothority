package sda

import (
	"testing"

	"reflect"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testServiceName = "testService"

func init() {
	RegisterNewService(testServiceName, newTestService)
	ServiceFactory.ServiceID(testServiceName)
	network.RegisterPacketType(&testMsg{})
}

func TestProcessor_AddMessage(t *testing.T) {
	h1 := NewLocalConode(2000)
	defer h1.Close()
	p := NewServiceProcessor(&Context{conode: h1})
	log.ErrFatal(p.RegisterMessage(procMsg))
	if len(p.handlers) != 1 {
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
	}
	for _, f := range wrongFunctions {
		fsig := reflect.TypeOf(f).String()
		log.Lvl2("Checking function", fsig)
		assert.Error(t, p.RegisterMessage(f),
			"Could register wrong function: "+fsig)
	}
}

func TestProcessor_RegisterMessages(t *testing.T) {
	h1 := NewLocalConode(2000)
	defer h1.Close()
	p := NewServiceProcessor(&Context{conode: h1})
	log.ErrFatal(p.RegisterMessages(procMsg, procMsg2))
	assert.Error(t, p.RegisterMessages(procMsg3, procMsgWrong4))
}

func TestServiceProcessor_ProcessClientRequest(t *testing.T) {
	h1 := NewLocalConode(2000)
	defer h1.Close()
	p := NewServiceProcessor(&Context{conode: h1})
	log.ErrFatal(p.RegisterMessage(procMsg))

	buf, err := protobuf.Encode(&testMsg{11})
	log.ErrFatal(err)
	rep, errCode := p.ProcessClientRequest("testMsg", buf)
	require.Equal(t, 0, errCode)
	val := &testMsg{}
	log.ErrFatal(protobuf.Decode(rep, val))
	if val.I != 11 {
		t.Fatal("Value got lost - should be 11")
	}

	buf, err = protobuf.Encode(&testMsg{42})
	log.ErrFatal(err)
	rep, errCode = p.ProcessClientRequest("testMsg", buf)
	require.Equal(t, 4142, errCode)
}

func TestProcessor_ProcessClientRequest(t *testing.T) {
	local := NewTCPTest()

	// generate 5 hosts,
	h := local.GenConodes(1)[0]
	defer local.CloseAll()

	client := local.NewClient(testServiceName)
	msg := &testMsg{}
	log.ErrFatal(client.SendProtobuf(h.ServerIdentity, &testMsg{12}, msg))
	if msg == nil {
		t.Fatal("Msg should not be nil")
	}
	if msg.I != 12 {
		t.Fatal("Didn't send 12")
	}

}

type testMsg struct {
	I int
}

type testMsg2 testMsg
type testMsg3 testMsg
type testMsg4 testMsg
type testMsg5 testMsg

func procMsg(msg *testMsg) (network.Body, int) {
	// Return an error for testing
	if msg.I == 42 {
		return nil, 4142
	}
	return msg, 0
}

func procMsg2(msg *testMsg2) (network.Body, int) {
	return nil, 0
}
func procMsg3(msg *testMsg3) (network.Body, int) {
	return nil, 0
}

func procMsgWrong1() (network.Body, int) {
	return nil, 0
}

func procMsgWrong2(msg testMsg2) (network.Body, int) {
	return msg, 0
}

func procMsgWrong3(msg *testMsg3) int {
	return 0
}

func procMsgWrong4(msg *testMsg4) (int, network.Body) {
	return 0, msg
}

type testService struct {
	*ServiceProcessor
	Msg interface{}
}

func newTestService(c *Context, path string) Service {
	ts := &testService{
		ServiceProcessor: NewServiceProcessor(c),
	}
	log.ErrFatal(ts.RegisterMessage(ts.ProcessMsg))
	return ts
}

func (ts *testService) NewProtocol(tn *TreeNodeInstance, conf *GenericConfig) (ProtocolInstance, error) {
	return nil, nil
}

func (ts *testService) ProcessMsg(msg *testMsg) (network.Body, int) {
	ts.Msg = msg
	return msg, 0
}
