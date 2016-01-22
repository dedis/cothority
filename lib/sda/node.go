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
	// channels holds all channels available for the different message-types
	channels map[uuid.UUID]interface{}
	// registered handler-functions for that protocol
	handlers map[uuid.UUID]MsgHandler
	// The protocolInstance belonging to that node
	instance ProtocolInstance
	// aggregate messages in order to dispatch them at once in the protocol
	// instance
	msgQueue map[uuid.UUID][]*SDAData
}

// MsgHandler is called upon reception of a certain message-type
type MsgHandler func([]*interface{})

// NewNode creates a new node
func NewNode(o *Overlay, tok *Token) (*Node, error) {
	n := &Node{overlay: o,
		token:    tok,
		channels: make(map[uuid.UUID]interface{}),
		handlers: make(map[uuid.UUID]MsgHandler),
		msgQueue: make(map[uuid.UUID][]*SDAData),
	}
	return n, n.protocolInstantiate()
}

// TreeNode gets the treeNode of this node
func (n *Node) TreeNode() *TreeNode {
	return n.Tree().GetTreeNode(n.token.TreeNodeID)
}

// Entity returns our entity
func (n *Node) Entity() *network.Entity {
	return n.TreeNode().Entity
}

// Parent returns the parent-TreeNode of ourselves
func (n *Node) Parent() *TreeNode {
	return n.TreeNode().Parent
}

// Children returns the children of ourselves
func (n *Node) Children() []*TreeNode {
	return n.TreeNode().Children
}

// Root returns the root-node of that tree
func (n *Node) Root() *TreeNode {
	return n.Tree().Root
}

// IsRoot returns whether whether we are at the top of the tree
func (n *Node) IsRoot() bool {
	return n.TreeNode().Parent == nil
}

// IsLeaf returns whether whether we are at the bottom of the tree
func (n *Node) IsLeaf() bool {
	return len(n.TreeNode().Children) == 0
}

// SendTo sends to a given node
func (n *Node) SendTo(to *TreeNode, msg interface{}) error {
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

// RegisterChannel takes a channel with a struct that contains two
// elements: a TreeNode and a message.
func (n *Node) RegisterChannel(c interface{}) error {
	cr := reflect.TypeOf(c)
	// Check we have the correct channel-type
	if cr.Kind() != reflect.Chan {
		return errors.New("Input is not channel")
	}
	if cr.Elem().Kind() != reflect.Struct {
		return errors.New("Input is not channel of structure")
	}
	if cr.Elem().NumField() != 2 {
		return errors.New("Input is not channel of structure with 2 elements")
	}
	dbg.Lvl3(cr.Elem().Field(0).Type)
	if cr.Elem().Field(0).Type != reflect.TypeOf(TreeNode{}) {
		return errors.New("Input-channel doesn't have TreeNode as element")
	}
	typ := network.RegisterMessageUUID(network.RTypeToUUID(cr.Elem().Field(1).Type),
		cr.Elem().Field(1).Type)
	n.channels[typ] = c
	dbg.Lvl3("Registered channel", typ)
	return nil
}

// ProtocolInstantiate creates a new instance of a protocol given by it's name
func (n *Node) protocolInstantiate() error {
	p, ok := protocols[n.token.ProtocolID]
	if !ok {
		return errors.New("Protocol doesn't exist")
	}
	tree := n.overlay.Tree(n.token.TreeID)
	if tree == nil {
		return errors.New("Tree does not exists")
	}
	if n.overlay.EntityList(n.token.EntityListID) == nil {
		return errors.New("EntityList does not exists")
	}
	if !n.TreeNode().IsInTree(tree) {
		return errors.New("We are not represented in the tree")
	}
	n.instance = p(n)
	return nil
}

func (n *Node) DispatchFunction(msg []*SDAData) error {
	dbg.Fatal("Not implemented for message", msg)
	return nil
}

// DispatchChannel takes a message and sends it to a channel
func (n *Node) DispatchChannel(msgSlice []*SDAData) error {
	for _, msg := range msgSlice {
		dbg.Lvl3("Received message of type:", msg.MsgType)
		out := n.channels[msg.MsgType]

		dbg.Lvl3("Making new", reflect.TypeOf(out))
		m := reflect.Indirect(reflect.New(reflect.TypeOf(out).Elem()))
		tn := n.Tree().GetTreeNode(msg.From.TreeNodeID)
		if tn == nil {
			return errors.New("Didn't find treenode")
		}

		m.Field(0).Set(reflect.ValueOf(*tn))
		m.Field(1).Set(reflect.ValueOf(msg.Msg))
		reflect.ValueOf(out).Send(m)
	}
	return nil
}

// DispatchMsg will dispatch this SDAData to the right instance
func (n *Node) DispatchMsg(sdaMsg *SDAData) error {
	// if message comes from parent, dispatch directly
	// if messages come from children we must aggregate them
	// if we still need to wait for additional messages, we return
	msgs, done := n.aggregate(sdaMsg)
	if !done {
		return nil
	}

	msgType := msgs[0].MsgType
	var err error
	switch {
	case n.channels[msgType] != nil:
		err = n.DispatchChannel(msgs)
	case n.handlers[msgType] != nil:
		err = n.DispatchFunction(msgs)
	default:
		err = n.instance.Dispatch(msgs)
	}
	return err
}

// aggregate store the message for a protocol instance such that a protocol
// instances will get all its children messages at once.
// node is the node the host is representing in this Tree, and sda is the
// message being analyzed.
func (n *Node) aggregate(sdaMsg *SDAData) ([]*SDAData, bool) {
	if !n.IsRoot() && uuid.Equal(sdaMsg.From.TreeNodeID, n.TreeNode().Parent.Id) {
		return []*SDAData{sdaMsg}, true
	}
	// store the msg
	tokId := sdaMsg.To.Id()
	if _, ok := n.msgQueue[tokId]; !ok {
		n.msgQueue[tokId] = make([]*SDAData, 0)
	}
	msgs := append(n.msgQueue[tokId], sdaMsg)
	n.msgQueue[tokId] = msgs
	// do we have everything yet or no
	// get the node this host is in this tree
	// OK we have all the children messages
	if len(msgs) == len(n.Children()) {
		// erase
		delete(n.msgQueue, tokId)
		return msgs, true
	}
	// no we still have to wait!
	dbg.Lvl2("Len(msg)=", len(msgs), "vs len(children)=", len(n.Children()))
	return nil, false
}

// Start calls the start-method on the protocol which in turn will initiate
// the first message to its children
func (n *Node) Start() error {
	return n.instance.Start()
}

// Private returns the corresponding private key
func (n *Node) Private() abstract.Secret {
	return n.overlay.host.private
}
