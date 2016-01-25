package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/satori/go.uuid"
	"testing"
	"time"
)

func TestNodeChannel(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	names := genLocalhostPeerNames(10, 2000)
	peerList := genEntityList(tSuite, names)
	// Generate an example topology
	tree := peerList.GenerateBinaryTree()
	dbg.Lvl4("Tree is", tree)
	h := sda.NewLocalHost(2000)
	defer h.Close()

	o := sda.NewOverlay(h)
	o.RegisterTree(tree)
	n, err := sda.NewNode(o, &sda.Token{TreeID: tree.Id})
	c := make(chan struct {
		sda.TreeNode
		NodeTestMsg
	}, 1)
	err = n.RegisterChannel(c)
	if err != nil {
		t.Fatal("Couldn't register channel:", err)
	}
	err = n.DispatchChannel([]*sda.SDAData{&sda.SDAData{
		Msg:     NodeTestMsg{3},
		MsgType: network.RegisterMessageType(NodeTestMsg{}),
		From: &sda.Token{
			TreeID:     tree.Id,
			TreeNodeID: tree.Root.Id,
		}},
	})
	if err != nil {
		t.Fatal("Couldn't dispatch to channel:", err)
	}
	msg := <-c
	if msg.I != 3 {
		t.Fatal("Message should contain '3'")
	}
}

// Test instantiation of Node
func TestNewNode(t *testing.T) {
	sda.ProtocolRegister(testID, NewProtocolTest)
	h1, h2 := SetupTwoHosts(t, false)
	// Add tree + entitylist
	el := sda.NewEntityList([]*network.Entity{h1.Entity, h2.Entity})
	h1.AddEntityList(el)
	tree := el.GenerateBinaryTree()
	h1.AddTree(tree)

	// Try directly StartNewNode
	node, err := h1.StartNewNode(testID, tree)
	if err != nil {
		t.Fatal("Could not start new protocol", err)
	}
	p := node.ProtocolInstance().(*ProtocolTest)
	if p.Msg != "Start" {
		t.Fatal("Start() not called - msg is:", p.Msg)
	}
	h1.Close()
	h2.Close()
}

func TestProtocolChannels(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	sda.ProtocolRegisterName("ProtoChannels", NewProtocolChannels)

	h1, h2 := SetupTwoHosts(t, true)
	defer h1.Close()
	defer h2.Close()
	// Add tree + entitylist
	el := sda.NewEntityList([]*network.Entity{h1.Entity, h2.Entity})
	h1.AddEntityList(el)
	tree := el.GenerateBinaryTree()
	h1.AddTree(tree)
	go h1.ProcessMessages()
	Incoming = make(chan struct {
		sda.TreeNode
		NodeTestMsg
	}, 2)

	// Try directly StartNewProtocol
	_, err := h1.StartNewNodeName("ProtoChannels", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}

	select {
	case msg := <-Incoming:
		if msg.I != 12 {
			t.Fatal("Child should receive 12")
		}
	case <-time.After(time.Second * 3):
		t.Fatal("Timeout")
	}
}

func TestMsgAggregation(t *testing.T) {
	local := sda.NewLocalTest()
	hosts, list, tree := local.GenTree(3, false, true)
	defer local.CloseAll()
	sda.ProtocolRegisterName("ProtoChannels", NewProtocolChannels)

	tok := &sda.Token{
		EntityListID: list.Id,
		TreeID:       tree.Id,
		TreeNodeID:   tree.Root.Id}
	// Two random types
	type1 := uuid.NewV4()
	type2 := uuid.NewV4()
	node, _ := sda.NewNode(hosts[0].Overlay(), tok)
	msg := &sda.SDAData{
		From:    tok.ChangeTreeNodeID(tree.Root.Children[0].Id),
		MsgType: type1,
		Msg:     nil,
	}

	msgType, _, done := node.Aggregate(msg)
	if done {
		t.Fatal("Should not be done for first message")
	}
	msg.From = tok.ChangeTreeNodeID(tree.Root.Children[1].Id)
	msgType, msgs, done := node.Aggregate(msg)
	if !done {
		t.Fatal("Should be done for the second message")
	}
	if len(msgs) != 2 {
		t.Fatal("Should have two messages")
	}
	if msgType != type1 {
		t.Fatal("Should have message of type1")
	}

	// Test checking of messages if they are different
	_, _, done = node.Aggregate(msg)
	if done {
		t.Fatal("Should not be done after first message")
	}
	msg.From = tok.ChangeTreeNodeID(tree.Root.Children[0].Id)
	msg.MsgType = type2
	_, _, done = node.Aggregate(msg)
	if done {
		t.Fatal("Should not be done after first message of new type")
	}

	msg.From = tok.ChangeTreeNodeID(tree.Root.Children[1].Id)
	msg.MsgType = type2
	_, _, done = node.Aggregate(msg)
	if !done {
		t.Fatal("Second message of type 2 should pass")
	}
	msg.From = tok.ChangeTreeNodeID(tree.Root.Children[0].Id)
	msg.MsgType = type1
	_, _, done = node.Aggregate(msg)
	if !done {
		t.Fatal("Second message of type 1 should pass")
	}

	// Test passing direct
	node.SetFlag(sda.BatchMessages)
	_, _, done = node.Aggregate(msg)
	if !done {
		t.Fatal("Now messages should pass directly")
	}
	time.Sleep(time.Millisecond * 100)
}

func TestFlags(t *testing.T) {
	n, _ := sda.NewNode(nil, nil)
	if n.HasFlag(sda.BatchMessages) {
		t.Fatal("Should NOT have batchMsgs-flag")
	}
	n.SetFlag(sda.BatchMessages)
	if !n.HasFlag(sda.BatchMessages) {
		t.Fatal("Should HAVE AggregateMessages-flag cleared")
	}
	n.ClearFlag(sda.BatchMessages)
	if n.HasFlag(sda.BatchMessages) {
		t.Fatal("Should NOT have AggregateMessages-flag")
	}
}

type NodeTestMsg struct {
	I int
}

var Incoming chan struct {
	sda.TreeNode
	NodeTestMsg
}

type ProtocolChannels struct {
	*sda.Node
}

func NewProtocolChannels(n *sda.Node) sda.ProtocolInstance {
	p := &ProtocolChannels{
		Node: n,
	}
	p.RegisterChannel(Incoming)
	return p
}

func (p *ProtocolChannels) Start() error {
	return p.SendTo(p.Children()[0], &NodeTestMsg{12})
}

func (p *ProtocolChannels) Dispatch([]*sda.SDAData) error {
	dbg.Error("This should not be called")
	return nil
}
