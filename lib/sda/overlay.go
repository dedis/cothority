package sda

import (
	"errors"
	"fmt"
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

type protocolMapper struct {
	// mapping instances with their tokens
	// maps token-uid to ProtocolInstances
	instances map[uuid.UUID]ProtocolInstance
	// aggregate messages in order to dispatch them at once in the protocol
	// instance
	msgQueue map[uuid.UUID][]*SDAData
	// Host reference
	Host *Host
}

func newProtocolMapper(h *Host) *protocolMapper {
	return &protocolMapper{
		instances: make(map[uuid.UUID]ProtocolInstance),
		msgQueue:  make(map[uuid.UUID][]*SDAData),
		Host:      h,
	}
}

// DispatchToInstance will dispatch this SDAData to the right instance
// it returns true if it has successfully dispatched the msg or false
// otherwise. It can return false because it want to aggregate some messages
// until every children of this host has sent their messages.
func (pm *protocolMapper) DispatchToInstance(sdaMsg *SDAData) (bool, error) {
	var pi ProtocolInstance
	if pi = pm.Instance(sdaMsg.To); pi == nil {
		return false, errors.New("No instance for this token")
	}
	//  Get the node corresponding to this host in the Tree
	node, err := pm.Host.TreeNodeFromToken(sdaMsg.To)
	if err != nil {
		return false, fmt.Errorf("Could not find TreeNode for this host in aggregate: %s", err)
	}
	// if message comes from parent, dispatch directly
	if !node.IsRoot() && sdaMsg.Entity.Equal(node.Parent.Entity) {
		return true, pi.Dispatch([]*SDAData{sdaMsg})
	}

	// if messages come from children we must aggregate them
	// if we still need to wait additionals message, we return
	msgs, ok := pm.aggregate(node, sdaMsg)
	if !ok {
		return false, nil
	}
	// all is good
	return true, pi.Dispatch(msgs)
}

// aggregate store the message for a protocol instance such that a protocol
// instances will get all its children messages at once.
// node is the node the host is representing in this Tree, and sda is the
// message being analyzed.
func (pm *protocolMapper) aggregate(node *TreeNode, sdaMsg *SDAData) ([]*SDAData, bool) {
	// store the msg
	tokId := sdaMsg.To.Id()
	if _, ok := pm.msgQueue[tokId]; !ok {
		pm.msgQueue[tokId] = make([]*SDAData, 0)
	}
	msgs := append(pm.msgQueue[tokId], sdaMsg)
	pm.msgQueue[tokId] = msgs
	// do we have everything yet or no
	// get the node this host is in this tree
	// OK we have all the children messages
	if len(msgs) == len(node.Children) {
		// erase
		delete(pm.msgQueue, tokId)
		return msgs, true
	}
	// no we still have to wait!
	dbg.Lvl2("Len(msg)=", len(msgs), "vs len(children)=", len(node.Children))
	return nil, false
}

// Instance returns the protocol instance associated with this token
// nil if not registered-
// Instance returns the protocol instance associated with this token
// nil if not registered.
func (pm *protocolMapper) Instance(tok *Token) ProtocolInstance {
	pi, _ := pm.instances[tok.Id()]
	return pi
}

// Exists returns true if a protocol instance exists (referenced its token ID)
func (pm *protocolMapper) Exists(tokenID uuid.UUID) bool {
	_, ok := pm.instances[tokenID]
	return ok
}

// RegisterProtocolInstance simply put the proto instance mapping with the token
func (pm *protocolMapper) RegisterProtocolInstance(proto ProtocolInstance, tok *Token) {
	// And registers it
	pm.instances[tok.Id()] = proto
}
