package sda_test

import (
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/satori/go.uuid"
	"testing"
	"time"
)

var testID = uuid.NewV5(uuid.NamespaceURL, "test")
var simpleID = uuid.NewV5(uuid.NamespaceURL, "simple")
var aggregateID = uuid.NewV5(uuid.NamespaceURL, "aggregate")

// ProtocolTest is the most simple protocol to be implemented, ignoring
// everything it receives.
type ProtocolTest struct {
	node *sda.Node
}

// NewProtocolTest is used to create a new protocolTest-instance
func NewProtocolTest(n *sda.Node) sda.ProtocolInstance {
	return &ProtocolTest{
		node: n,
	}
}

// Dispatch is used to send the messages further - here everything is
// copied to /dev/null
func (p *ProtocolTest) Dispatch(m []*sda.SDAData) error {
	dbg.Lvl2("PRotocolTest.Dispatch()")
	return nil
}

func (p *ProtocolTest) Start() error {
	dbg.Lvl2("ProtocolTest.Start()")
	return nil
}

type SimpleProtocol struct {
	// chan to get back to testing
	Chan chan bool
	node *sda.Node
}

// Sends a simple message to its first children
func (p *SimpleProtocol) Start() error {
	dbg.Lvl2("Sending from", p.node.Entity().First(), "to",
		p.node.Children()[0].Entity.First())
	err := p.node.SendTo(p.node.Children()[0], &SimpleMessage{10})
	if err != nil {
		return err
	}
	p.Chan <- true
	return nil
}

// Dispatch analyses the message and does nothing else
func (p *SimpleProtocol) Dispatch(m []*sda.SDAData) error {
	dbg.Lvl2("Dispatching", m)
	if m[0].MsgType != SimpleMessageType {
		return errors.New("Not the message expected")
	}
	msg := m[0].Msg.(SimpleMessage)
	if msg.I != 10 {
		return errors.New("Not the value expected")
	}
	p.Chan <- true
	return nil
}

// Test simple protocol-implementation
// - registration
func TestProtocolRegistration(t *testing.T) {
	if sda.ProtocolExists(testID) {
		t.Fatal("Test should not exist yet")
	}
	sda.ProtocolRegister(testID, NewProtocolTest)
	if !sda.ProtocolExists(testID) {
		t.Fatal("Test should exist now")
	}
}

var testString = ""

// This makes h2 the leader, so it creates a tree and entity list
// and start a protocol. H1 should receive that message and request the entitity
// list and the treelist and then instantiate the protocol.
func TestProtocolAutomaticInstantiation(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	// setup
	chanH1 := make(chan bool)
	chanH2 := make(chan bool)
	chans := []chan bool{chanH1, chanH2}
	id := 0
	// custom creation function so we know the step due to the channels
	fn := func(n *sda.Node) sda.ProtocolInstance {
		ps := SimpleProtocol{
			node: n,
			Chan: chans[id],
		}
		id++
		return &ps
	}

	sda.ProtocolRegister(testID, fn)
	h1, h2 := setupHosts(t, true)
	defer h1.Close()
	defer h2.Close()
	go h1.ProcessMessages()
	// create small Tree
	el := sda.NewEntityList([]*network.Entity{h1.Entity, h2.Entity})
	h1.AddEntityList(el)
	tree, _ := el.GenerateBinaryTree()
	h1.AddTree(tree)
	// start the protocol
	go func() {
		_, err := h1.StartNewNode(testID, tree)
		if err != nil {
			t.Fatal(fmt.Sprintf("Could not start protocol %v", err))
		}
	}()

	// we are supposed to receive something from host1 from Start()
	select {
	case _ = <-chanH1:
		break
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Could not receive from channel of host 1")
	}
	// Then we are supposed to receive from h2 after he got the tree and the
	// entity list from h1
	select {
	case _ = <-chanH2:
		break
	case <-time.After(2 * time.Second):
		t.Fatal("Could not receive from channel of host 1")
	}
	// then it's all good
}

// Test if protocol aggregate children well or not
func TestProtocolAggregation(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	ch1 := make(chan bool)
	chroot := make(chan bool)
	ch2 := make(chan bool)
	chans := []chan bool{chroot, ch1, ch2}
	id := 0
	// custom creation function so we know the step due to the channels
	fn := func(n *sda.Node) sda.ProtocolInstance {
		ps := AggregationProtocol{
			Node: n,
			Chan: chans[id],
		}
		id++
		return &ps
	}

	sda.ProtocolRegister(aggregateID, fn)
	hosts := GenHosts(t, 3)
	root := hosts[0]
	// create small Tree
	el := sda.NewEntityList([]*network.Entity{root.Entity, hosts[1].Entity, hosts[2].Entity})
	root.AddEntityList(el)
	tree, _ := el.GenerateBinaryTree()
	root.AddTree(tree)
	// start a protocol
	go func() {
		_, err := root.StartNewNode(aggregateID, tree)
		if err != nil {
			t.Fatal(fmt.Sprintf("Could not start protocol %v", err))
		}
	}()
	// we are supposed to receive stg from host2 from Start()
	select {
	case _ = <-chroot:
		break
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Could not receive from channel of root ")
	}
	// Then we are supposed to receive from h1 after he got the tree and the
	// entity list from h2
	var foundh1 bool
	var foundh2 bool
	select {
	case _ = <-ch1:
		foundh1 = true
		if foundh2 {
			break
		}
	case _ = <-ch2:
		foundh2 = true
		if foundh1 {
			break
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Could not receive from channel of host 1")
	}
	// then the root is supposed to get them all
	select {
	case _ = <-chroot:
		break
	case <-time.After(1 * time.Second):
		t.Fatal("Could not receive message from root in the protocol")
	}
	// then it's all good
	root.Close()
	hosts[1].Close()
	hosts[2].Close()

}

type AggregationProtocol struct {
	*sda.Node
	// chan to get back to testing
	Chan chan bool
}

// Dispatch simply analysze the message and do nothing else
func (p *AggregationProtocol) Dispatch(ms []*sda.SDAData) error {
	// with one lvl tree, only root is waiting
	dbg.Lvl3("Entity is", p.Entity())
	if p.IsRoot() && len(ms) != len(p.Children()) {
		// testing will fail then
		return fmt.Errorf("aggregation wrong number of children")
	}
	if p.IsLeaf() {
		m := ms[0].Msg.(SimpleMessage)
		dbg.Lvl2("Aggregationprotocol children sending message up")
		// send back to parent
		if err := p.SendTo(p.Parent(), &m); err != nil {
			return fmt.Errorf("Sending to parent failed")
		}
	}
	p.Chan <- true
	return nil
}

// Sends a simple message to its first children
func (p *AggregationProtocol) Start() error {
	msg := SimpleMessage{10}
	for _, c := range p.Children() {
		dbg.Lvl3("Sending to child", c.Entity.First())
		if err := p.SendTo(c, &msg); err != nil {
			return fmt.Errorf("Could not send to children %v", err)
		}
	}
	p.Chan <- true
	return nil
}
