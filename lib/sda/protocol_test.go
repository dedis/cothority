package sda_test

import (
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

func (p *ProtocolTest) Start() {
	dbg.Lvl2("ProtocolTest.Start()")
	testString = p.id.String()
}

type SimpleProtocol struct {
	*sda.Host
	*sda.TreeNode
	id  uuid.UUID
	tok *sda.Token
	// chan to get back to testing
	Chan chan bool
}

func (p *SimpleProtocol) Id() uuid.UUID {
	return p.id
}

// Dispatch simply analysze the message and do nothing else
func (p *SimpleProtocol) Dispatch(m []*sda.SDAData) error {
	if m[0].MsgType != SimpleMessageType {
		return fmt.Errorf("Not the message expected")
	}
	msg := m[0].Msg.(SimpleMessage)
	if msg.I != 10 {
		return fmt.Errorf("Not the value expected")
	}
	p.Chan <- true
	return nil
}

// Sends a simple message to its first children
func (p *SimpleProtocol) Start() {
	msg := SimpleMessage{10}
	child := p.Children[0]
	p.Send(p.tok, child.Entity, &msg)
	p.Chan <- true

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
func aTestProtocolAutomaticInstantiation(t *testing.T) {
	// setup
	chanH1 := make(chan bool)
	chanH2 := make(chan bool)
	chans := []chan bool{chanH2, chanH1}
	id := 0
	// custom creation function so we know the step due to the channels
	fn := func(h *sda.Host, tr *sda.TreeNode, tok *sda.Token) sda.ProtocolInstance {
		uid, _ := uuid.FromString(strconv.Itoa(id))
		ps := SimpleProtocol{
			id:       uid,
			Host:     h,
			TreeNode: tr,
			tok:      tok,
			Chan:     chans[id],
		}
		id++
		return &ps
	}

	sda.ProtocolRegister(testID, fn)
	h1, h2 := setupHosts(t, true)
	go h1.ProcessMessages()
	// create small Tree
	el := sda.NewEntityList([]*network.Entity{h2.Entity, h1.Entity})
	h2.AddEntityList(el)
	tree, _ := el.GenerateBinaryTree()
	h2.AddTree(tree)
	// start a protocol
	go func() {

		_, err := h2.StartNewProtocol(testID, tree.Id)
		if err != nil {
			t.Fatal(fmt.Sprintf("Could not start protocol %v", err))
		}
	}()
	// we are supposed to receive stg from host2 from Start()
	select {
	case _ = <-chanH2:
		break
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Could not receive from channel of host 2")
	}
	// Then we are supposed to receive from h1 after he got the tree and the
	// entity list from h2
	select {
	case _ = <-chanH1:
		break
	case <-time.After(2 * time.Second):
		t.Fatal("Could not receive from channel of host 1")
	}
	// then it's all good
	h1.Close()
	h2.Close()
}
