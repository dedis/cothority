package sda

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/satori/go.uuid"
)

/*
Overlay keeps all trees and entity-lists for a given host. It creates
Nodes and ProtocolInstances upon request and dispatches the messages.
*/

type Overlay struct {
	host        *Host
	nodes       map[Token]*Node
	trees       map[uuid.UUID]*Tree
	entityLists map[uuid.UUID]*EntityList
}

// NewOverlay creates a new overlay-structure
func NewOverlay(h *Host) *Overlay {
	return &Overlay{
		host:        h,
		nodes:       make(map[Token]*Node),
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
	if !ProtocolExists(sdaMsg.To.ProtocolID) {
		return errors.New("Protocol does not exists from token")
	}
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
	// If pi does not exists, then instantiate it !
	if !o.host.mapper.Exists(sdaMsg.To.Id()) {
		_, err := o.host.protocolInstantiate(sdaMsg.To, tree.GetTreeNode(sdaMsg.To.TreeNodeID))
		if err != nil {
			return err
		}
	}
	_, err := o.host.mapper.DispatchToInstance(sdaMsg)
	if err != nil {
		return err
	}
	return nil

	//	return o.DispatchToInstance(sdaMsg)
}

func (o *Overlay) DispatchToInstance(msg *SDAData) error {

	node, ok := o.nodes[*(msg.To)]
	if !ok {
		// Create the node
		o.nodes[*(msg.To)] = NewNode(o, msg.To)
		return o.TransmitMsg(msg)
	}
	node.DispatchChannel(msg)
	return nil
}

// SendTo takes a destination and a message to send.
func (o *Overlay) SendTo(from *Token, dest *TreeNode, msg interface{}) error {
	return nil
}

// RegisterTree takes a tree and puts it in the map
func (o *Overlay) RegisterTree(t *Tree) {
	o.trees[t.Id] = t
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
