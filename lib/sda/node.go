package sda

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
	"reflect"
)

/*
Node represents a protocol-instance in a given TreeNode. It is linked to
Overlay where all the tree-structures are stored.
*/

type Node struct {
	overlay *Overlay
	token   *Token
	// cache for the TreeNode this Node is representing
	treeNode *TreeNode
	// channels holds all channels available for the different message-types
	channels map[uuid.UUID]interface{}
	// registered handler-functions for that protocol
	handlers map[uuid.UUID]interface{}
	// flags for messages - only one channel/handler possible
	messageTypeFlags map[uuid.UUID]uint32
	// The protocolInstance belonging to that node
	instance ProtocolInstance
	// aggregate messages in order to dispatch them at once in the protocol
	// instance
	msgQueue map[uuid.UUID][]*SDAData
	// done channel
	done chan bool
	// done callback
	onDoneCallback func() bool
}

// Bit-values for different messageTypeFlags
// If AggregateMessages is set, messages from all children are collected
// before sent to the Node
// https://golang.org/ref/spec#Iota
const (
	AggregateMessages = 1 << iota
)

// MsgHandler is called upon reception of a certain message-type
type MsgHandler func([]*interface{})

// NewNode creates a new node
func NewNode(o *Overlay, tok *Token) (*Node, error) {
	n := NewNodeEmpty(o, tok)
	return n, n.protocolInstantiate()
}

// NewNodeEmpty creates a new node without a protocol
func NewNodeEmpty(o *Overlay, tok *Token) *Node {
	n := &Node{overlay: o,
		token:            tok,
		channels:         make(map[uuid.UUID]interface{}),
		handlers:         make(map[uuid.UUID]interface{}),
		msgQueue:         make(map[uuid.UUID][]*SDAData),
		messageTypeFlags: make(map[uuid.UUID]uint32),
		treeNode:         nil,
		done:             make(chan bool),
	}
	return n
}

// TreeNode gets the treeNode of this node. If there is no TreeNode for the
// Token of this node, the function will return nil
func (n *Node) TreeNode() *TreeNode {
	return n.treeNode
}

// Entity returns our entity
func (n *Node) Entity() *network.Entity {
	return n.treeNode.Entity
}

// Parent returns the parent-TreeNode of ourselves
func (n *Node) Parent() *TreeNode {
	return n.treeNode.Parent
}

// Children returns the children of ourselves
func (n *Node) Children() []*TreeNode {
	return n.treeNode.Children
}

// Root returns the root-node of that tree
func (n *Node) Root() *TreeNode {
	return n.Tree().Root
}

// IsRoot returns whether whether we are at the top of the tree
func (n *Node) IsRoot() bool {
	return n.treeNode.Parent == nil
}

// IsLeaf returns whether whether we are at the bottom of the tree
func (n *Node) IsLeaf() bool {
	return len(n.treeNode.Children) == 0
}

// SendTo sends to a given node
func (n *Node) SendTo(to *TreeNode, msg interface{}) error {
	if to == nil {
		return errors.New("Sent to a nil TreeNode")
	}
	// Check whether we know that type of message
	mr := reflect.ValueOf(msg)
	m := reflect.Indirect(mr).Type()
	t := network.RTypeToUUID(m)
	_, has_c := n.channels[t]
	_, has_h := n.handlers[t]
	if !has_c && !has_h {
		dbg.Print(m, t)
		return errors.New("Didn't find message-type in either channels or handlers")
	}
	dbg.Print(mr.Elem().Field(0).Type())
	dbg.Print(mr.Elem().Field(0).CanSet())
	mr.Elem().Field(0).Set(reflect.New(reflect.TypeOf(TreeNode{})))

	return n.overlay.SendToTreeNode(n.token, to, msg)
}

// Tree returns the tree of that node
func (n *Node) Tree() *Tree {
	return n.overlay.TreeFromToken(n.token)
}

// EntityList returns the entity-list
func (n *Node) EntityList() *EntityList {
	return n.Tree().EntityList
}

func (n *Node) Suite() abstract.Suite {
	return n.overlay.Suite()
}

// RegisterChannel takes a channel with a struct that contains two
// elements: a TreeNode and a message. It will send every message that are the
// same type to this channel.
// This function handles also
// - registration of the message-type
// - aggregation or not of messages: if you give a channel of slices, the
//   messages will be aggregated, else they will come one-by-one
func (n *Node) RegisterChannel(c interface{}) error {
	flags := uint32(0)
	cr := reflect.TypeOf(c)
	if cr.Kind() == reflect.Ptr {
		val := reflect.ValueOf(c).Elem()
		val.Set(reflect.MakeChan(val.Type(), 1))
		//val.Set(reflect.MakeChan(reflect.Indirect(cr), 1))
		return n.RegisterChannel(reflect.Indirect(val).Interface())
	} else if reflect.ValueOf(c).IsNil() {
		return errors.New("Can not Register a (value) channel not initialized")
	}
	// Check we have the correct channel-type
	if cr.Kind() != reflect.Chan {
		return errors.New("Input is not channel")
	}
	if cr.Elem().Kind() == reflect.Slice {
		flags += AggregateMessages
		cr = cr.Elem()
	}
	if cr.Elem().Kind() != reflect.Struct {
		return errors.New("Input is not channel of structure")
	}
	if cr.Elem().Field(0).Type != reflect.TypeOf(&TreeNode{}) {
		return errors.New("Input-channel doesn't have *TreeNode as 1st element")
	}
	// Automatic registration of the message to the network library.
	typ := network.RegisterMessageUUID(network.RTypeToUUID(cr.Elem()),
		cr.Elem())
	//typ := network.RTypeToUUID(cr.Elem().Field(1).Type)
	n.channels[typ] = c
	n.messageTypeFlags[typ] = flags
	dbg.Lvl3("Registered channel", typ, "with flags", flags)
	return nil
}

// RegisterChannel takes a channel with a struct that contains two
// elements: a TreeNode and a message. It will send every message that are the
// same type to this channel.
// This function handles also
// - registration of the message-type
// - aggregation or not of messages: if you give a channel of slices, the
//   messages will be aggregated, else they will come one-by-one
func (n *Node) RegisterHandler(c interface{}) error {
	flags := uint32(0)
	cr := reflect.TypeOf(c)
	// Check we have the correct channel-type
	if cr.Kind() != reflect.Func {
		return errors.New("Input is not function")
	}
	cr = cr.In(0)
	if cr.Kind() == reflect.Slice {
		flags += AggregateMessages
		cr = cr.Elem()
	}
	if cr.Kind() != reflect.Struct {
		return errors.New("Input is not channel of structure")
	}
	if cr.Field(0).Type != reflect.TypeOf(&TreeNode{}) {
		return errors.New("Input-channel doesn't have TreeNode as element")
	}
	// Automatic registration of the message to the network library.
	typ := network.RegisterMessageUUID(network.RTypeToUUID(cr.Field(1).Type),
		cr.Field(1).Type)
	//typ := network.RTypeToUUID(cr.Elem().Field(1).Type)
	n.handlers[typ] = c
	n.messageTypeFlags[typ] = flags
	dbg.Lvl3("Registered handler", typ, "with flags", flags)
	return nil
}

// ProtocolInstance returns the instance of the running protocol
func (n *Node) ProtocolInstance() ProtocolInstance {
	return n.instance
}

// ProtocolInstantiate creates a new instance of a protocol given by it's name
func (n *Node) protocolInstantiate() error {
	if n.token == nil {
		return errors.New("Hope this is running in test-mode")
	}
	pid := n.token.ProtocolID
	p, ok := protocols[pid]
	if !ok {
		return errors.New("Protocol " + pid.String() + " doesn't exist")
	}
	tree := n.overlay.Tree(n.token.TreeID)
	if tree == nil {
		return errors.New("Tree does not exists")
	}
	if n.overlay.EntityList(n.token.EntityListID) == nil {
		return errors.New("EntityList does not exists")
	}
	var err error
	n.treeNode, err = n.overlay.TreeNodeFromToken(n.token)
	if err != nil {
		return errors.New("We are not represented in the tree")
	}
	n.instance, err = p(n)
	if err == nil {
		go n.instance.Dispatch()
	}
	return err
}

// Dispatch - the standard dispatching function is empty
func (n *Node) Dispatch() error {
	return nil
}

func (n *Node) DispatchHandler(msgSlice []*SDAData) error {
	mt := msgSlice[0].MsgType
	to := reflect.TypeOf(n.handlers[mt]).In(0)
	f := reflect.ValueOf(n.handlers[mt])
	if n.HasFlag(mt, AggregateMessages) {
		msgs := reflect.MakeSlice(to, len(msgSlice), len(msgSlice))
		for i, msg := range msgSlice {
			msgs.Index(i).Set(n.ReflectCreate(to.Elem(), msg))
		}
		dbg.Lvl4("Dispatching aggregation to", n.Entity().Addresses)
		f.Call([]reflect.Value{msgs})
	} else {
		for _, msg := range msgSlice {
			dbg.Lvl4("Dispatching to", n.Entity().Addresses)
			m := n.ReflectCreate(to, msg)
			f.Call([]reflect.Value{m})
		}
	}
	return nil
}

func (n *Node) ReflectCreate(t reflect.Type, msg *SDAData) reflect.Value {
	m := reflect.ValueOf(msg.Msg)
	dbg.Print(m.Field(0).Type())
	tn := n.Tree().GetTreeNode(msg.From.TreeNodeID)
	if tn != nil {
		dbg.Print(m.Field(0).Elem().CanSet())
		m.Field(0).Elem().Set(reflect.ValueOf(*tn))
	}
	return m
}

// DispatchChannel takes a message and sends it to a channel
func (n *Node) DispatchChannel(msgSlice []*SDAData) error {
	mt := msgSlice[0].MsgType
	to := reflect.TypeOf(n.channels[mt])
	if n.HasFlag(mt, AggregateMessages) {
		dbg.Lvl4("Received aggregated message of type:", mt)
		to = to.Elem()
		out := reflect.MakeSlice(to, len(msgSlice), len(msgSlice))
		for i, msg := range msgSlice {
			dbg.Lvl4("Dispatching aggregated to", to)
			m := n.ReflectCreate(to.Elem(), msg)
			dbg.Lvl4("Adding msg", m, "to", n.Entity().Addresses)
			out.Index(i).Set(m)
		}
		reflect.ValueOf(n.channels[mt]).Send(out)
	} else {
		dbg.Lvl3("Received message of type:", mt)
		for _, msg := range msgSlice {
			out := n.channels[mt]

			m := n.ReflectCreate(to.Elem(), msg)
			dbg.Lvl4("Dispatching msg type", mt, " to", to, " :", m.Field(1).Interface())
			reflect.ValueOf(out).Send(m)
		}
	}
	return nil
}

// DispatchMsg will dispatch this SDAData to the right instance
func (n *Node) DispatchMsg(sdaMsg *SDAData) error {
	// if message comes from parent, dispatch directly
	// if messages come from children we must aggregate them
	// if we still need to wait for additional messages, we return
	msgType, msgs, done := n.aggregate(sdaMsg)
	if !done {
		dbg.Lvl3("Not done")
		return nil
	}
	dbg.Lvl4("Going to dispatch", sdaMsg)

	var err error
	switch {
	case n.channels[msgType] != nil:
		dbg.Lvl4("Dispatching to channel")
		err = n.DispatchChannel(msgs)
	case n.handlers[msgType] != nil:
		dbg.Lvl4("Dispatching to handler", n.Entity().Addresses)
		err = n.DispatchHandler(msgs)
	default:
		return errors.New("This message-type is not handled by this protocol")
	}
	return err
}

// SetFlag makes sure a given flag is set
func (n *Node) SetFlag(mt uuid.UUID, f uint32) {
	n.messageTypeFlags[mt] |= f
}

// ClearFlag makes sure a given flag is removed
func (n *Node) ClearFlag(mt uuid.UUID, f uint32) {
	n.messageTypeFlags[mt] &^= f
}

// HasFlag returns true if the given flag is set
func (n *Node) HasFlag(mt uuid.UUID, f uint32) bool {
	return n.messageTypeFlags[mt]&f != 0
}

// aggregate store the message for a protocol instance such that a protocol
// instances will get all its children messages at once.
// node is the node the host is representing in this Tree, and sda is the
// message being analyzed.
func (n *Node) aggregate(sdaMsg *SDAData) (uuid.UUID, []*SDAData, bool) {
	mt := sdaMsg.MsgType
	fromParent := !n.IsRoot() && uuid.Equal(sdaMsg.From.TreeNodeID, n.Parent().Id)
	if fromParent || !n.HasFlag(mt, AggregateMessages) {
		return mt, []*SDAData{sdaMsg}, true
	}
	// store the msg according to its type
	if _, ok := n.msgQueue[mt]; !ok {
		n.msgQueue[mt] = make([]*SDAData, 0)
	}
	msgs := append(n.msgQueue[mt], sdaMsg)
	n.msgQueue[mt] = msgs
	dbg.Lvl4(n.Entity().Addresses, "received", len(msgs), "of", len(n.Children()), "messages")

	// do we have everything yet or no
	// get the node this host is in this tree
	// OK we have all the children messages
	if len(msgs) == len(n.Children()) {
		// erase
		delete(n.msgQueue, mt)
		return mt, msgs, true
	}
	// no we still have to wait!
	return mt, nil, false
}

// Start calls the start-method on the protocol which in turn will initiate
// the first message to its children
func (n *Node) Start() error {
	return n.instance.Start()
}

// Done returns a channel that must be given a bool when a protocol instance has
// finished its work.
func (n *Node) Done() {
	if n.onDoneCallback != nil {
		ok := n.onDoneCallback()
		if !ok {
			return
		}
	}
	n.overlay.nodeDone(n.token)
}

// OnDoneCallback should be called if we want to control the Done() of the node.
// It is used by protocols that uses others protocols inside and that want to
// control when the final Done() should be called.
// the function should return true if the real Done() has to be called otherwise
// false.
func (n *Node) OnDoneCallback(fn func() bool) {
	n.onDoneCallback = fn
}

// Private returns the corresponding private key
func (n *Node) Private() abstract.Secret {
	return n.overlay.host.private
}

// Closes the host
func (n *Node) Close() error {
	return n.overlay.host.Close()
}
