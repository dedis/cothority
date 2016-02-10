package sda

import (
	"errors"
	"fmt"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
)

/*
Overlay keeps all trees and entity-lists for a given host. It creates
Nodes and ProtocolInstances upon request and dispatches the messages.
*/

type Overlay struct {
	host *Host
	// mapping from Token.Id() to Node
	nodes map[uuid.UUID]*Node
	// mapping from Tree.Id to Tree
	trees map[uuid.UUID]*Tree
	// mapping from EntityList.id to EntityList
	entityLists map[uuid.UUID]*EntityList
	// cache for relating token(~Node) to TreeNode
	cache TreeNodeCache
}

// NewOverlay creates a new overlay-structure
func NewOverlay(h *Host) *Overlay {
	return &Overlay{
		host:        h,
		nodes:       make(map[uuid.UUID]*Node),
		trees:       make(map[uuid.UUID]*Tree),
		entityLists: make(map[uuid.UUID]*EntityList),
		cache:       NewTreeNodeCache(),
	}
}

// TransmitMsg takes a message received from the host and treats it. It might
// - ask for the identityList
// - ask for the Tree
// - create a new protocolInstance
// - pass it to a given protocolInstance
func (o *Overlay) TransmitMsg(sdaMsg *SDAData) error {
	dbg.Lvl5(o.host.Entity.Addresses, "got message to transmit:", sdaMsg)
	// do we have the entitylist ? if not, ask for it.
	if o.EntityList(sdaMsg.To.EntityListID) == nil {
		dbg.Lvl2("Will ask for entityList from token")
		return o.host.requestTree(sdaMsg.Entity, sdaMsg)
	}
	tree := o.Tree(sdaMsg.To.TreeID)
	if tree == nil {
		dbg.Lvl3("Will ask for tree from token")
		return o.host.requestTree(sdaMsg.Entity, sdaMsg)
	}
	// If node does not exists, then create it
	node := o.nodes[sdaMsg.To.Id()]
	if node == nil {
		dbg.Lvl2("Node not found for token (creating new one):", fmt.Sprintf("%+v", sdaMsg.To))
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

	// instantiate node
	var err error
	var node *Node
	node, err = o.CreateNewNode(protocolID, tree)
	if err != nil {
		return nil, err
	}
	// start it
	dbg.Lvl3("Starting new node at", o.host.Entity.Addresses)
	err = node.Start()
	if err != nil {
		return nil, err
	}
	return node, nil
}

// CreateNewNode is used when you want to create the node with the protocol
// instance but do not want to start it yet. Use case are when you are root, you
// want to specifiy some additional configuration for example.
func (o *Overlay) CreateNewNode(protocolID uuid.UUID, tree *Tree) (*Node, error) {
	node, err := o.NewNodeEmpty(protocolID, tree)
	if err != nil {
		return nil, err
	}
	o.nodes[node.token.Id()] = node
	return node, node.protocolInstantiate()
}

func (o *Overlay) StartNewNodeName(name string, tree *Tree) (*Node, error) {
	return o.StartNewNode(ProtocolNameToUuid(name), tree)
}

// CreateNewNodeName only creates the Node but do not call the instantiation of
// the protocol directly, that way you can do your own stuff before calling
// protocol.Start() or node.Start()
func (o *Overlay) CreateNewNodeName(name string, tree *Tree) (*Node, error) {
	return o.CreateNewNode(ProtocolNameToUuid(name), tree)
}

func (o *Overlay) NewNodeEmptyName(name string, tree *Tree) (*Node, error) {
	return o.NewNodeEmpty(ProtocolNameToUuid(name), tree)
}

// NewNode returns a simple node without instantiating anything no protocol.
func (o *Overlay) NewNodeEmpty(protocolID uuid.UUID, tree *Tree) (*Node, error) {
	// check everything exists
	if !ProtocolExists(protocolID) {
		return nil, errors.New("Protocol doesn't exists: " + protocolID.String())
	}
	rootEntity := tree.Root.Entity
	if !o.host.Entity.Equal(rootEntity) {
		return nil, errors.New("StartNewNode should be called by root, but entity of host differs from the root")
	}
	// instantiate
	token := &Token{
		ProtocolID:   protocolID,
		EntityListID: tree.EntityList.Id,
		TreeID:       tree.Id,
		TreeNodeID:   tree.Root.Id,
		// Host is handling the generation of protocolInstanceID
		RoundID: uuid.NewV4(),
	}
	node := NewNodeEmpty(o, token)
	o.nodes[node.token.Id()] = node
	return node, nil
}

// TreeNodeFromToken returns the treeNode corresponding to a token
func (o *Overlay) TreeNodeFromToken(t *Token) (*TreeNode, error) {
	// First, check the cache
	if tn := o.cache.GetFromToken(t); tn != nil {
		return tn, nil
	}
	// If cache has not, then search the tree
	tree := o.Tree(t.TreeID)
	if tree == nil {
		return nil, errors.New("Didn't find tree")
	}
	tn := tree.GetTreeNode(t.TreeNodeID)
	if tn == nil {
		return nil, errors.New("Didn't find treenode")
	}
	// Since we found treeNode, cache it so later reuse
	o.cache.Cache(tree, tn)
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

// nodeDone is called by node to signify that its work is finished and its
// ressources can be released
func (o *Overlay) nodeDone(tok *Token) {
	delete(o.nodes, tok.Id())
}

func (o *Overlay) Private() abstract.Secret {
	return o.host.Private()
}
func (o *Overlay) Suite() abstract.Suite {
	return o.host.Suite()
}

// TreeNodeCache is a cache that maps from token to treeNode. Since the mapping
// is not 1-1 (many Token can point to one TreeNode, but one token leads to one
// TreeNode), we have to do certain
// lookup, but that's better than searching the tree each time.
type TreeNodeCache map[uuid.UUID]map[uuid.UUID]*TreeNode

// Returns a new TreeNodeCache
func NewTreeNodeCache() TreeNodeCache {
	m := make(map[uuid.UUID]map[uuid.UUID]*TreeNode)
	return m
}

// Cache a TreeNode that relates to the Tree
// It will also cache the parent and children of the treenode since that's most
// likely what we are going to query.
func (tnc TreeNodeCache) Cache(tree *Tree, treeNode *TreeNode) {
	var mm map[uuid.UUID]*TreeNode
	var ok bool
	if mm, ok = tnc[tree.Id]; !ok {
		mm = make(map[uuid.UUID]*TreeNode)
	}
	// add treenode
	mm[treeNode.Id] = treeNode
	// add parent if not root
	if treeNode.Parent != nil {
		mm[treeNode.Parent.Id] = treeNode.Parent
	}
	// add children
	for _, c := range treeNode.Children {
		mm[c.Id] = c
	}
	// add cache
	tnc[tree.Id] = mm
}

// GetFromToken returns the TreeNode that the token is pointing at, or
// nil if there is none for this token.
func (tnc TreeNodeCache) GetFromToken(tok *Token) *TreeNode {
	var mm map[uuid.UUID]*TreeNode
	var ok bool
	if mm, ok = tnc[tok.TreeID]; !ok {
		// no tree cached for this token :...
		return nil
	}
	var tn *TreeNode
	if tn, ok = mm[tok.TreeNodeID]; !ok {
		// no treeNode cached for this token...
		// XXX Should we search the tree ? Then we need to keep reference to the
		// tree ...
		return nil
	}
	return tn
}
