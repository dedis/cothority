package sda

import (
	"errors"
	"fmt"
	"testing"

	"github.com/alecthomas/assert"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
)

var testProto = "test"

var simpleProto = "simple"

// ProtocolTest is the most simple protocol to be implemented, ignoring
// everything it receives.
type ProtocolTest struct {
	*TreeNodeInstance
	StartMsg chan string
	DispMsg  chan string
}

// NewProtocolTest is used to create a new protocolTest-instance
func NewProtocolTest(n *TreeNodeInstance) (ProtocolInstance, error) {
	return &ProtocolTest{
		TreeNodeInstance: n,
		StartMsg:         make(chan string, 1),
		DispMsg:          make(chan string),
	}, nil
}

// Dispatch is used to send the messages further - here everything is
// copied to /dev/null
func (p *ProtocolTest) Dispatch() error {
	log.Lvl2("ProtocolTest.Dispatch()")
	p.DispMsg <- "Dispatch"
	return nil
}

func (p *ProtocolTest) Start() error {
	log.Lvl2("ProtocolTest.Start()")
	p.StartMsg <- "Start"
	return nil
}

type SimpleProtocol struct {
	// chan to get back to testing
	Chan chan bool
	*TreeNodeInstance
}

// Sends a simple message to its first children
func (p *SimpleProtocol) Start() error {
	err := p.SendTo(p.Children()[0], &SimpleMessage{10})
	if err != nil {
		return err
	}
	p.Chan <- true
	return nil
}

// Dispatch analyses the message and does nothing else
func (p *SimpleProtocol) ReceiveMessage(msg struct {
	*TreeNode
	SimpleMessage
}) error {
	if msg.I != 10 {
		return errors.New("Not the value expected")
	}
	p.Chan <- true
	return nil
}

type SimpleMessage struct {
	I int
}

// Test simple protocol-implementation
// - registration
func TestProtocolRegistration(t *testing.T) {
	testProtoName := "testProto"
	testProtoID, err := GlobalProtocolRegister(testProtoName, NewProtocolTest)
	log.ErrFatal(err)
	_, err = GlobalProtocolRegister(testProtoName, NewProtocolTest)
	require.NotNil(t, err)
	if !protocols.ProtocolExists(testProtoID) {
		t.Fatal("Test should exist now")
	}
	if ProtocolNameToID(testProtoName) != testProtoID {
		t.Fatal("Not correct translation from string to ID")
	}
	require.Equal(t, "", protocols.ProtocolIDToName(ProtocolID(uuid.Nil)))
	if protocols.ProtocolIDToName(testProtoID) != testProtoName {
		t.Fatal("Not correct translation from ID to String")
	}
}

// This makes h2 the leader, so it creates a tree and entity list
// and start a protocol. H1 should receive that message and request the entitity
// list and the treelist and then instantiate the protocol.
func TestProtocolAutomaticInstantiation(t *testing.T) {
	// setup
	chanH1 := make(chan bool)
	chanH2 := make(chan bool)
	chans := []chan bool{chanH1, chanH2}
	id := 0
	// custom creation function so we know the step due to the channels
	fn := func(n *TreeNodeInstance) (ProtocolInstance, error) {
		ps := SimpleProtocol{
			TreeNodeInstance: n,
			Chan:             chans[id],
		}
		ps.RegisterHandler(ps.ReceiveMessage)
		id++
		return &ps, nil
	}

	network.RegisterPacketType(SimpleMessage{})
	GlobalProtocolRegister(simpleProto, fn)
	local := NewLocalTest()
	defer local.CloseAll()
	h, _, tree := local.GenTree(2, true)
	h1 := h[0]
	// start the protocol
	go func() {
		_, err := h1.StartProtocol(simpleProto, tree)
		if err != nil {
			t.Fatal(fmt.Sprintf("Could not start protocol %v", err))
		}
	}()

	// we are supposed to receive something from host1 from Start()
	<-chanH1

	// Then we are supposed to receive from h2 after he got the tree and the
	// entity list from h1
	<-chanH2
}

func TestProtocolIOFactory(t *testing.T) {
	defer eraseAllProtocolIO()
	RegisterProtocolIO(NewTestProtocolIOChan)
	assert.True(t, len(protocolIOFactory.factories) == 1)
}

func TestProtocolIOStore(t *testing.T) {
	defer eraseAllProtocolIO()
	local := NewLocalTest()
	defer local.CloseAll()

	RegisterProtocolIO(NewTestProtocolIO)
	GlobalProtocolRegister(testProtoIOName, newTestProtocolInstance)
	h, _, tree := local.GenTree(2, true)

	go func() {
		// first time to wrap
		res := <-chanProtoIOFeedback
		require.Equal(t, "", res)
		// second time to unwrap
		res = <-chanProtoIOFeedback
		require.Equal(t, "", res)

	}()
	_, err := h[0].StartProtocol(testProtoIOName, tree)
	require.Nil(t, err)

	res := <-chanTestProtoInstance
	assert.True(t, res)
}

// ProtocolIO part
var chanProtoIOCreation = make(chan bool)
var chanProtoIOFeedback = make(chan string)

const testProtoIOName = "TestIO"

type OuterPacket struct {
	Info  *OverlayMessage
	Inner *SimpleMessage
}

var OuterPacketType = network.RegisterPacketType(OuterPacket{})

type TestProtocolIO struct{}

func NewTestProtocolIOChan() ProtocolIO {
	chanProtoIOCreation <- true
	return &TestProtocolIO{}
}

func NewTestProtocolIO() ProtocolIO {
	return &TestProtocolIO{}
}

func eraseAllProtocolIO() {
	protocolIOFactory.factories = nil
}

func (t *TestProtocolIO) Wrap(msg interface{}, info *OverlayMessage) (interface{}, error) {
	outer := &OuterPacket{}
	inner, ok := msg.(*SimpleMessage)
	if !ok {
		chanProtoIOFeedback <- "wrong message type in wrap"
	}
	outer.Inner = inner
	outer.Info = info
	chanProtoIOFeedback <- ""
	return outer, nil
}

func (t *TestProtocolIO) Unwrap(msg interface{}) (interface{}, *OverlayMessage, error) {
	if msg == nil {
		chanProtoIOFeedback <- "message nil!"
		return nil, nil, errors.New("message nil")
	}

	real, ok := msg.(OuterPacket)
	if !ok {
		chanProtoIOFeedback <- "wrong type of message in unwrap"
		return nil, nil, errors.New("wrong message")
	}
	chanProtoIOFeedback <- ""
	return real.Inner, real.Info, nil
}

func (t *TestProtocolIO) PacketType() network.PacketTypeID {
	return OuterPacketType
}

func (t *TestProtocolIO) Name() string {
	return testProtoIOName
}

var chanTestProtoInstance = make(chan bool)

// ProtocolInstance part
type TestProtocolInstance struct {
	*TreeNodeInstance
}

func newTestProtocolInstance(n *TreeNodeInstance) (ProtocolInstance, error) {
	pi := &TestProtocolInstance{n}
	n.RegisterHandler(pi.handleSimpleMessage)
	return pi, nil
}

func (t *TestProtocolInstance) Start() error {
	t.SendTo(t.Root(), &SimpleMessage{12})
	return nil
}

type SimpleMessageHandler struct {
	*TreeNode
	SimpleMessage
}

func (t TestProtocolInstance) handleSimpleMessage(h SimpleMessageHandler) error {
	chanTestProtoInstance <- h.SimpleMessage.I == 12
	return nil
}
