package sda

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
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
}

// NewNode creates a new node
func NewNode(o *Overlay, tok *Token) *Node {
	return &Node{overlay: o,
		token:    tok,
		channels: make(map[uuid.UUID]interface{}),
	}
}

// TreeNode gets the treeNode of this node
func (n *Node) TreeNode() *TreeNode {
	return n.Tree().GetNode(n.token.TreeNodeID)
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

// SendTo sends to a given node
func (n *Node) SendTo(to *TreeNode, msg interface{}) error {
	return n.overlay.SendTo(n.token, to, msg)
}

// Tree returns the tree of that node
func (n *Node) Tree() *Tree {
	return n.overlay.Tree(n.token)
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

// DispatchChannel takes a message and sends it to a channel
func (n *Node) DispatchChannel(msg *SDAData) error {
	typ := reflect.TypeOf(msg.Msg)
	dbg.Lvl3("Received message of type:", msg.MsgType)
	out, ok := n.channels[msg.MsgType]
	if !ok {
		dbg.Lvl3("Calling Dispatch as message is not known:", typ)
	}

	dbg.Lvl3("Making new", reflect.TypeOf(out))
	m := reflect.Indirect(reflect.New(reflect.TypeOf(out).Elem()))
	tn := n.Tree().GetNode(msg.From.TreeNodeID)
	if tn == nil {
		return errors.New("Didn't find treenode")
	}

	m.Field(0).Set(reflect.ValueOf(*tn))
	m.Field(1).Set(reflect.ValueOf(msg.Msg))
	reflect.ValueOf(out).Send(m)
	return nil
}
