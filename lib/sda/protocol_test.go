package sda_test

import (
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/satori/go.uuid"
	"strconv"
	"testing"
	"time"
)

var testID = uuid.NewV5(uuid.NamespaceURL, "test")
var simpleID = uuid.NewV5(uuid.NamespaceURL, "simple")
var aggregateID = uuid.NewV5(uuid.NamespaceURL, "aggregate")

// ProtocolTest is the most simple protocol to be implemented, ignoring
// everything it receives.
type ProtocolTest struct {
	*sda.Host
	*sda.TreeNode
	id  uuid.UUID
	tok *sda.Token
}

var currInstanceID int

// NewProtocolTest is used to create a new protocolTest-instance
func NewProtocolTest(n *sda.Host, t *sda.TreeNode, tok *sda.Token) sda.ProtocolInstance {
	currInstanceID++
	url := "http://dedis.epfl.ch/protocol/test/" + strconv.Itoa(currInstanceID)
	return &ProtocolTest{
		Host:     n,
		TreeNode: t,
		id:       uuid.NewV5(uuid.NamespaceURL, url),
		tok:      tok,
	}
}

func (p *ProtocolTest) Id() uuid.UUID {
	return p.id
}

// Dispatch is used to send the messages further - here everything is
// copied to /dev/null
func (p *ProtocolTest) Dispatch(m []*sda.SDAData) error {
	dbg.Lvl2("PRotocolTest.Dispatch()")
	return nil
}

func (p *ProtocolTest) Start() error {
	dbg.Lvl2("ProtocolTest.Start()")
	testString = p.id.String()
	return nil
}

type SimpleProtocol struct {
	*sda.ProtocolStruct
	// chan to get back to testing
	Chan chan bool
}

// Dispatch simply analyse the message and do nothing else
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

// Sends a simple message to its first children
func (p *SimpleProtocol) Start() error {
	dbg.Lvl2("Sending from", p.TreeNode.Entity.Addresses, "to",
		p.Children[0].Entity.Addresses)
	err := p.Send(p.Children[0], &SimpleMessage{10})
	if err != nil {
		return err
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

// Test instantiation of the protocol
func TestProtocolInstantiation(t *testing.T) {
	sda.ProtocolRegister(testID, NewProtocolTest)
	h1, h2 := setupHosts(t, false)
	// Add tree + entitylist
	//el := GenEntityListFrom(h1.Suite(), genLocalhostPeerNames(10, 2000))
	el := sda.NewEntityList([]*network.Entity{h2.Entity, h1.Entity})
	h1.AddEntityList(el)
	tree, _ := el.GenerateBinaryTree()
	h1.AddTree(tree)
	// Then try to instantiate
	tok := &sda.Token{
		ProtocolID:   testID,
		TreeID:       tree.Id,
		EntityListID: tree.EntityList.Id,
	}

	p, err := h1.ProtocolInstantiate(tok, tree.Root)
	if err != nil {
		t.Fatal("Couldn't instantiate test-protocol")
	}
	if p.Dispatch(nil) != nil {
		t.Fatal("Dispatch-method didn't return nil")
	}

	// Try directly StartNewProtocol
	_, err = h1.StartNewProtocol(testID, tree.Id)
	if err != nil {
		t.Fatal("Could not start new protocol")
	}
	if testString == "" {
		t.Fatal("Start() not called")
	}
	h1.Close()
	h2.Close()
}

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
	fn := func(h *sda.Host, tr *sda.TreeNode, tok *sda.Token) sda.ProtocolInstance {
		ps := SimpleProtocol{
			ProtocolStruct: sda.NewProtocolStruct(h, tr, tok),
			Chan:           chans[id],
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
		_, err := h1.StartNewProtocol(testID, tree.Id)
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
	ch1 := make(chan bool)
	chroot := make(chan bool)
	ch2 := make(chan bool)
	chans := []chan bool{chroot, ch1, ch2}
	id := 0
	// custom creation function so we know the step due to the channels
	fn := func(h *sda.Host, tr *sda.TreeNode, tok *sda.Token) sda.ProtocolInstance {
		ps := AggregationProtocol{
			Host:     h,
			TreeNode: tr,
			tok:      tok,
			Chan:     chans[id],
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
		_, err := root.StartNewProtocol(aggregateID, tree.Id)
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
	*sda.Host
	*sda.TreeNode
	tok *sda.Token
	// chan to get back to testing
	Chan chan bool
}

// Dispatch simply analysze the message and do nothing else
func (p *AggregationProtocol) Dispatch(ms []*sda.SDAData) error {
	tn := p.TreeNode
	// with one lvl tree, only root is waiting
	if tn.IsRoot() && len(ms) != len(tn.Children) {
		// testing will fail then
		return fmt.Errorf("aggregation wrong number of children")
	}
	if tn.IsLeaf() {
		m := ms[0].Msg.(SimpleMessage)
		dbg.Lvl2("Aggregationprotocol children sending message up")
		// send back to parent
		if err := p.SendToTreeNode(p.tok, tn.Parent, &m); err != nil {
			return fmt.Errorf("Sending to parent failed")
		}
	}
	p.Chan <- true
	return nil
}

// Sends a simple message to its first children
func (p *AggregationProtocol) Start() error {
	msg := SimpleMessage{10}
	tn := p.TreeNode
	for i := range tn.Children {
		if err := p.SendToTreeNode(p.tok, tn.Children[i], &msg); err != nil {
			return fmt.Errorf("Could not send to children %v", err)
		}
	}
	p.Chan <- true
	return nil
}
