package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/satori/go.uuid"
	"reflect"
	"testing"
	"time"
)

func init() {
	sda.ProtocolRegisterName("ProtocolChannels", NewProtocolChannels)
	sda.ProtocolRegister(testID, NewProtocolTest)
	Incoming = make(chan struct {
		sda.TreeNode
		NodeTestMsg
	})
}

func TestReflectChannel(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	var c chan bool
	cp := &c

	//ty := reflect.TypeOf(cp)
	v := reflect.ValueOf(cp).Elem()
	dbg.Lvl3(v.CanSet())
	ty := v.Type()
	dbg.Lvl3(ty)
	v.Set(reflect.MakeChan(ty, 1))
	c <- true
	return
	/*
		dbg.Print(reflect.TypeOf(cp).Kind())
		dbg.Print(reflect.TypeOf(c).Kind())
		dbg.Print(reflect.TypeOf(reflect.Indirect(reflect.ValueOf(cp)).Interface()).Kind())
		dbg.Print(reflect.ValueOf(c).IsNil())
		dbg.Print(reflect.ValueOf(c).Cap())
		dbg.Print(reflect.ValueOf(c).Len())
		c = make(chan struct {
			sda.TreeNode
			NodeTestMsg
		}, 1)
		dbg.Print(reflect.ValueOf(c).IsValid())
		dbg.Print(reflect.ValueOf(c).IsNil())
		dbg.Print(reflect.ValueOf(c).Cap())
		dbg.Print(reflect.ValueOf(c).Len())
	*/
}

func TestNodeChannelCreateSlice(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	n := sda.NewNodeEmpty(nil, nil)
	var c chan []struct {
		sda.TreeNode
		NodeTestMsg
	}
	err := n.RegisterChannel(&c)
	if err != nil {
		t.Fatal("Couldn't register channel:", err)
	}
}

func TestNodeChannelCreate(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(2, false, true)
	defer local.CloseAll()

	n, err := local.NewNode(tree.Root, "ProtocolChannels")
	if err != nil {
		t.Fatal("Couldn't create new node:", err)
	}
	var c chan struct {
		sda.TreeNode
		NodeTestMsg
	}
	err = n.RegisterChannel(&c)
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

func TestNodeChannel(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(2, false, true)
	defer local.CloseAll()

	n, err := local.NewNode(tree.Root, "ProtocolChannels")
	if err != nil {
		t.Fatal("Couldn't create new node:", err)
	}
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
	h1, h2 := SetupTwoHosts(t, true)
	defer h1.Close()
	defer h2.Close()
	// Add tree + entitylist
	el := sda.NewEntityList([]*network.Entity{h1.Entity, h2.Entity})
	h1.AddEntityList(el)
	tree := el.GenerateBinaryTree()
	h1.AddTree(tree)
	go h1.ProcessMessages()

	// Try directly StartNewProtocol
	_, err := h1.StartNewNodeName("ProtocolChannels", tree)
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
	_, _, tree := local.GenTree(3, false, true)
	defer local.CloseAll()
	root, err := local.StartNewNodeName("ProtocolChannels", tree)
	if err != nil {
		t.Fatal("Couldn't create new node:", err)
	}
	proto := root.ProtocolInstance().(*ProtocolChannels)
	// Wait for both children to be up
	<-Incoming
	<-Incoming
	dbg.Lvl3("Both children are up")
	child1 := local.GetNodes(tree.Root.Children[0])[0]
	child2 := local.GetNodes(tree.Root.Children[1])[0]

	local.SendTreeNode("ProtocolChannels", child1, root, &NodeTestAggMsg{3})
	if len(proto.IncomingAgg) > 0 {
		t.Fatal("Messages should NOT be there")
	}
	local.SendTreeNode("ProtocolChannels", child2, root, &NodeTestAggMsg{4})
	if len(proto.IncomingAgg) == 0 {
		t.Fatal("Messages should BE there")
	}
	msgs := <-proto.IncomingAgg
	if msgs[0].I != 3 {
		t.Fatal("First message should be 3")
	}
	if msgs[1].I != 4 {
		t.Fatal("Second message should be 4")
	}
}

func TestFlags(t *testing.T) {
	n, _ := sda.NewNode(nil, nil)
	if n.HasFlag(uuid.Nil, sda.AggregateMessages) {
		t.Fatal("Should NOT have AggregateMessages-flag")
	}
	n.SetFlag(uuid.Nil, sda.AggregateMessages)
	if !n.HasFlag(uuid.Nil, sda.AggregateMessages) {
		t.Fatal("Should HAVE AggregateMessages-flag cleared")
	}
	n.ClearFlag(uuid.Nil, sda.AggregateMessages)
	if n.HasFlag(uuid.Nil, sda.AggregateMessages) {
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

type NodeTestAggMsg struct {
	I int
}

type ProtocolChannels struct {
	*sda.Node
	IncomingAgg chan []struct {
		sda.TreeNode
		NodeTestAggMsg
	}
}

func NewProtocolChannels(n *sda.Node) (sda.ProtocolInstance, error) {
	p := &ProtocolChannels{
		Node: n,
	}
	p.RegisterChannel(Incoming)
	p.RegisterChannel(&p.IncomingAgg)
	return p, nil
}

func (p *ProtocolChannels) Start() error {
	for _, c := range p.Children() {
		err := p.SendTo(c, &NodeTestMsg{12})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *ProtocolChannels) Dispatch([]*sda.SDAData) error {
	dbg.Error("This should not be called")
	return nil
}

// relese ressources ==> call Done()
func (p *ProtocolChannels) Release() {
	p.Done()
}
