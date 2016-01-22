package sda

import (
	"errors"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
)

/*
Overlay keeps all trees and entity-lists for a given host. It creates
Nodes and ProtocolInstances upon request and dispatches the messages.
*/

type Overlay struct {
	host        *Host
	nodes       map[uuid.UUID]*Node
	trees       map[uuid.UUID]*Tree
	entityLists map[uuid.UUID]*EntityList
}

// NewOverlay creates a new overlay-structure
func NewOverlay(h *Host) *Overlay {
	return &Overlay{
		host:        h,
		nodes:       make(map[uuid.UUID]*Node),
		trees:       make(map[uuid.UUID]*Tree),
		entityLists: make(map[uuid.UUID]*EntityList),
	}
}

// TransmitMsg takes a message received from the host and treats it. It might
// - ask for the identityList
// - ask for the Tree
// - create a new protocolInstance
// - pass it to a given protocolInstance
func (o *Overlay) TransmitMsg(sdaMsg *SDAData) error {
	dbg.Lvl4("Got message to transmit:", sdaMsg)
	// do we have the entitylist ? if not, ask for it.
	if o.EntityList(sdaMsg.To.EntityListID) == nil {
		dbg.Lvl2("Will ask for entityList + tree from token")
		return o.host.requestTree(sdaMsg.Entity, sdaMsg)
	}
	tree := o.Tree(sdaMsg.To.TreeID)
	if tree == nil {
		dbg.Lvl2("Will ask for tree from token")
		return o.host.requestTree(sdaMsg.Entity, sdaMsg)
	}
	// If node does not exists, then create it
	node := o.nodes[sdaMsg.To.Id()]
	if node == nil {
		var err error
		o.nodes[sdaMsg.To.Id()], err = NewNode(o, sdaMsg.To)
		if err != nil {
			return err
		}
		node = o.nodes[sdaMsg.To.Id()]
	}
	err := node.DispatchMsg(sdaMsg)
	if err != nil {
		return err
	}
	return nil
}

// RegisterTree takes a tree and puts it in the map
func (o *Overlay) RegisterTree(t *Tree) {
	o.trees[t.Id] = t
	o.host.checkPendingSDA(t)
}

// TreeFromToken searches for the tree corresponding to a token.
func (o *Overlay) TreeFromToken(tok *Token) *Tree {
	return o.trees[tok.TreeID]
}

// Tree returns the tree given by treeId or nil if not found
func (o *Overlay) Tree(tid uuid.UUID) *Tree {
	return o.trees[tid]
}

// RegisterEntityList puts an entityList in the map
func (o *Overlay) RegisterEntityList(el *EntityList) {
	o.entityLists[el.Id] = el
}

// EntityListFromToken returns the entitylist corresponding to a token
func (o *Overlay) EntityListFromToken(tok *Token) *EntityList {
	return o.entityLists[tok.EntityListID]
}

// EntityList returns the entityList given by EntityListID
func (o *Overlay) EntityList(elid uuid.UUID) *EntityList {
	return o.entityLists[elid]
}

// StartNewNode starts a new node which will in turn instantiate the desired
// protocol. This is called from the root-node and will start the
// protocol
func (o *Overlay) StartNewNode(protocolID uuid.UUID, tree *Tree) (*Node, error) {
	// check everything exists
	if !ProtocolExists(protocolID) {
		return nil, errors.New("Protocol does not exists")
	}

	// instantiate
	token := &Token{
		ProtocolID:   protocolID,
		EntityListID: tree.EntityList.Id,
		TreeID:       tree.Id,
		TreeNodeID:   tree.Root.Id,
		// Host is handling the generation of protocolInstanceID
		RoundID: cliutils.NewRandomUUID(),
	}
	// instantiate node
	var err error
	o.nodes[token.Id()], err = NewNode(o, token)
	if err != nil {
		return nil, err
	}

	// start it
	dbg.Lvl3("Starting new node at", o.host.Entity.Addresses)
	err = o.nodes[token.Id()].Start()
	if err != nil {
		return nil, err
	}
	return o.nodes[token.Id()], nil
}

func (o *Overlay) StartNewNodeName(name string, tree *Tree) (*Node, error) {
	return o.StartNewNode(ProtocolNameToUuid(name), tree)
}

// TreeNodeFromToken returns the treeNode corresponding to a token
func (o *Overlay) TreeNodeFromToken(t *Token) (*TreeNode, error) {
	tree := o.Tree(t.TreeID)
	if tree == nil {
		return nil, errors.New("Didn't find tree")
	}
	tn := tree.GetTreeNode(t.TreeNodeID)
	if tn == nil {
		return nil, errors.New("Didn't find treenode")
	}
	return tn, nil
}

// SendToTreeNode sends a message to a treeNode
func (o *Overlay) SendToTreeNode(from *Token, to *TreeNode, msg network.ProtocolMessage) error {
	sda := &SDAData{
		Msg:  msg,
		From: from,
		To:   from.ChangeTreeNodeID(to.Id),
	}
	return o.host.sendSDAData(to.Entity, sda)
}

// SendToToken is the main function protocol instance must use in order to send a
// message across the network.
func (o *Overlay) SendToToken(from, to *Token, msg network.ProtocolMessage) error {
	if from == nil {
		return errors.New("From-token is nil")
	}
	if to == nil {
		return errors.New("To-token is nil")
	}
	if o.nodes[from.Id()] == nil {
		return errors.New("No protocol instance registered with this token.")
	}
	tn, err := o.TreeNodeFromToken(to)
	if err != nil {
		return errors.New("Didn't find TreeNode for token: " + err.Error())
	}
	return o.SendToTreeNode(from, tn, msg)
}
