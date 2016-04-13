package sda

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
)

// Overlay keeps all trees and entity-lists for a given host. It creates
// Nodes and ProtocolInstances upon request and dispatches the messages.
type Overlay struct {
	host *Host
	// mapping from Token.Id() to Node
	nodes map[TokenID]*Node
	// false = NOT DONE
	// true = DONE
	nodeInfo map[TokenID]bool
	nodeLock sync.Mutex
	// mapping from Tree.Id to Tree
	trees    map[TreeID]*Tree
	treesMut sync.Mutex
	// mapping from EntityList.id to EntityList
	entityLists    map[EntityListID]*EntityList
	entityListLock sync.Mutex
	// cache for relating token(~Node) to TreeNode
	cache TreeNodeCache

	// TreeNodeInstance part
	instances     map[TokenID]*TreeNodeInstance
	instancesInfo map[TokenID]bool
	instancesLock sync.Mutex
	pis           map[TokenID]ProtocolInstance
}

// NewOverlay creates a new overlay-structure
func NewOverlay(h *Host) *Overlay {
	return &Overlay{
		host:          h,
		nodes:         make(map[TokenID]*Node),
		nodeInfo:      make(map[TokenID]bool),
		trees:         make(map[TreeID]*Tree),
		entityLists:   make(map[EntityListID]*EntityList),
		cache:         NewTreeNodeCache(),
		instances:     make(map[TokenID]*TreeNodeInstance),
		instancesInfo: make(map[TokenID]bool),
		pis:           make(map[TokenID]ProtocolInstance),
	}
}

// TransmitMsg takes a message received from the host and treats it. It might
// - ask for the identityList
// - ask for the Tree
// - create a new protocolInstance
// - pass it to a given protocolInstance
func (o *Overlay) TransmitMsg(sdaMsg *Data) error {
	// do we have the entitylist ? if not, ask for it.
	if o.EntityList(sdaMsg.To.EntityListID) == nil {
		dbg.Lvl3("Will ask the EntityList from token", sdaMsg.To.EntityListID, len(o.entityLists), o.host.workingAddress)
		return o.host.requestTree(sdaMsg.Entity, sdaMsg)
	}
	tree := o.Tree(sdaMsg.To.TreeID)
	if tree == nil {
		dbg.Lvl3("Will ask for tree from token")
		return o.host.requestTree(sdaMsg.Entity, sdaMsg)
	}
	// TreeNodeInstance
	var pi ProtocolInstance
	o.instancesLock.Lock()
	pi, ok := o.pis[sdaMsg.To.Id()]
	o.instancesLock.Unlock()
	// if the TreeNodeInstance is not there, creates it
	if !ok {
		tn, err := o.TreeNodeFromToken(sdaMsg.To)
		if err != nil {
			return errors.New("No TreeNode defined in this tree here")
		}
		tni := o.newTreeNodeInstanceFromToken(tn, sdaMsg.To)
		// see if we know the Service Recipient
		s, ok := o.host.serviceStore.serviceByID(sdaMsg.To.ServiceID)

		// no servies defined => check if there is a protocol that can be
		// created
		if !ok {
			pi, err = ProtocolInstantiate(sdaMsg.To.ProtoID, tni)
			if err != nil {
				return err
			}
			go pi.Dispatch()

			/// use the Services to instantiate it
		} else {
			tn, err := o.TreeNodeFromToken(sdaMsg.To)
			if err != nil {
				return errors.New("No TreeNode defined in this tree here")
			}
			tni := o.newTreeNodeInstanceFromToken(tn, sdaMsg.To)
			// request the PI from the Service and bind the two
			pi, err = s.NewProtocol(tni, &sdaMsg.Config)
			if err != nil {
				return err
			}
		}
		if err := o.RegisterProtocolInstance(pi); err != nil {
			return errors.New("Error Binding TNI and PI")
		}
		dbg.Lvl2(o.host.workingAddress, "Overlay created new ProtocolInstace msg => ", fmt.Sprintf("%+v", sdaMsg.To))

	}
	// TODO Check if TNI is already Done
	pi.DispatchMsg(sdaMsg)

	//// If node does not exists, then create it
	//o.nodeLock.Lock()
	//node := o.nodes[sdaMsg.To.Id()]
	//isDone := o.nodeInfo[sdaMsg.To.Id()]
	//// If we never have seen this token before, then we create it
	//if node == nil && !isDone {
	//dbg.Lvl3(o.host.Entity.First(), "creating new node for token:", sdaMsg.To.Id())
	//var err error
	//o.nodes[sdaMsg.To.Id()], err = NewNode(o, sdaMsg.To)
	//o.nodeInfo[sdaMsg.To.Id()] = false
	//if err != nil {
	//o.nodeLock.Unlock()
	//return err
	//}
	//node = o.nodes[sdaMsg.To.Id()]
	//}
	//// If node is ALREADY DONE => drop packet
	//if isDone {
	//dbg.Lvl2("Dropped message given to node that is done.")
	//o.nodeLock.Unlock()
	//return nil
	//}
	//o.nodeLock.Unlock()
	/*node.DispatchMsg(sdaMsg)*/
	return nil
}

// RegisterTree takes a tree and puts it in the map
func (o *Overlay) RegisterTree(t *Tree) {
	o.treesMut.Lock()
	o.trees[t.Id] = t
	o.treesMut.Unlock()
	o.host.checkPendingSDA(t)
}

// TreeFromToken searches for the tree corresponding to a token.
func (o *Overlay) TreeFromToken(tok *Token) *Tree {
	return o.trees[tok.TreeID]
}

// Tree returns the tree given by treeId or nil if not found
func (o *Overlay) Tree(tid TreeID) *Tree {
	o.treesMut.Lock()
	defer o.treesMut.Unlock()
	return o.trees[tid]
}

// RegisterEntityList puts an entityList in the map
func (o *Overlay) RegisterEntityList(el *EntityList) {
	o.entityListLock.Lock()
	defer o.entityListLock.Unlock()
	o.entityLists[el.Id] = el
}

// EntityListFromToken returns the entitylist corresponding to a token
func (o *Overlay) EntityListFromToken(tok *Token) *EntityList {
	return o.entityLists[tok.EntityListID]
}

// EntityList returns the entityList given by EntityListID
func (o *Overlay) EntityList(elid EntityListID) *EntityList {
	o.entityListLock.Lock()
	defer o.entityListLock.Unlock()
	return o.entityLists[elid]
}

// StartNewNode starts a new node which will in turn instantiate the desired
// protocol. This is called from the root-node and will start the
// protocol.
func (o *Overlay) StartNewNode(protocolID ProtocolID, tree *Tree) (*Node, error) {

	// instantiate node
	var err error
	var node *Node
	node, err = o.CreateNewNode(protocolID, tree)
	if err != nil {
		return nil, err
	}
	// start it
	dbg.Lvl3("Starting new node at", o.host.Entity.Addresses)
	node.StartProtocol()
	return node, nil
}

// CreateNewNode is used when you want to create the node with the protocol
// instance but do not want to start it yet. Use case are when you are root, you
// want to specifiy some additional configuration for example.
func (o *Overlay) CreateNewNode(protocolID ProtocolID, tree *Tree) (*Node, error) {
	node, err := o.NewNodeEmpty(protocolID, tree)
	if err != nil {
		return nil, err
	}
	o.nodeLock.Lock()
	defer o.nodeLock.Unlock()
	o.nodes[node.token.Id()] = node
	o.nodeInfo[node.token.Id()] = false
	return node, node.protocolInstantiate()
}

// StartNewNodeName starts a node from the given protocol name. Can be useful
// in simulations. See for instance the protocol "CloseAll".
func (o *Overlay) StartNewNodeName(name string, tree *Tree) (*Node, error) {
	return o.StartNewNode(ProtocolNameToID(name), tree)
}

// CreateNewNodeName only creates the Node but do not call the instantiation of
// the protocol directly, that way you can do your own stuff before calling
// protocol.Start() or node.Start()
func (o *Overlay) CreateNewNodeName(name string, tree *Tree) (*Node, error) {
	return o.CreateNewNode(ProtocolNameToID(name), tree)
}

// NewNodeEmptyName returns a simple node from the protocol name but without
// instantiating the protocol.
func (o *Overlay) NewNodeEmptyName(name string, tree *Tree) (*Node, error) {
	return o.NewNodeEmpty(ProtocolNameToID(name), tree)
}

// NewNode returns a simple node without instantiating anything no protocol.
func (o *Overlay) NewNodeEmpty(protocolID ProtocolID, tree *Tree) (*Node, error) {
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
		ProtoID:      protocolID,
		EntityListID: tree.EntityList.Id,
		TreeID:       tree.Id,
		TreeNodeID:   tree.Root.Id,
		// Host is handling the generation of protocolInstanceID
		RoundID: RoundID(uuid.NewV4()),
	}
	o.nodeLock.Lock()
	defer o.nodeLock.Unlock()
	node, err := NewNodeEmpty(o, token)
	o.nodes[node.token.Id()] = node
	o.nodeInfo[node.token.Id()] = false
	return node, err
}

// TreeNodeFromToken returns the treeNode corresponding to a token
func (o *Overlay) TreeNodeFromToken(t *Token) (*TreeNode, error) {
	if t == nil {
		return nil, errors.New("Didn't find tree-node: No token given.")
	}
	// First, check the cache
	if tn := o.cache.GetFromToken(t); tn != nil {
		return tn, nil
	}
	// If cache has not, then search the tree
	tree := o.Tree(t.TreeID)
	if tree == nil {
		return nil, errors.New("Didn't find tree")
	}
	tn := tree.Search(t.TreeNodeID)
	if tn == nil {
		return nil, errors.New("Didn't find treenode")
	}
	// Since we found treeNode, cache it so later reuse
	o.cache.Cache(tree, tn)
	return tn, nil
}

// SendToTreeNode sends a message to a treeNode
func (o *Overlay) SendToTreeNode(from *Token, to *TreeNode, msg network.ProtocolMessage) error {
	sda := &Data{
		Msg:  msg,
		From: from,
		To:   from.ChangeTreeNodeID(to.Id),
	}
	dbg.Lvl4("Sending to entity", to.Entity.Addresses)
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
	o.nodeLock.Lock()
	if o.nodes[from.Id()] == nil {
		o.nodeLock.Unlock()
		return errors.New("No protocol instance registered with this token.")
	}
	o.nodeLock.Unlock()
	tn, err := o.TreeNodeFromToken(to)
	if err != nil {
		return errors.New("Didn't find TreeNode for token: " + err.Error())
	}
	return o.SendToTreeNode(from, tn, msg)
}

// nodeDone is called by node to signify that its work is finished and its
// ressources can be released
func (o *Overlay) nodeDone(tok *Token) {
	o.nodeLock.Lock()
	defer o.nodeLock.Unlock()
	o.nodeDelete(tok)
}

// nodeDelete needs to be separated from nodeDone, as it is also called from
// Close, but due to locking-issues here we don't lock.
func (o *Overlay) nodeDelete(tok *Token) {
	node, ok := o.nodes[tok.Id()]
	if !ok {
		dbg.Lvl2("Node", tok.Id(), "already gone")
		return
	}
	dbg.Lvl4("Closing node", tok.Id())
	err := node.Close()
	if err != nil {
		dbg.Error("Error while closing node:", err)
	}
	delete(o.nodes, tok.Id())
	// mark it done !
	o.nodeInfo[tok.Id()] = true
}

func (o *Overlay) suite() abstract.Suite {
	return o.host.Suite()
}

// Close calls all nodes, deletes them from the list and closes them
func (o *Overlay) Close() {
	o.nodeLock.Lock()
	defer o.nodeLock.Unlock()
	for _, n := range o.nodes {
		dbg.Lvl4("Closing node", n.TokenID())
		o.nodeDelete(n.Token())
	}

	o.instancesLock.Lock()
	for _, tni := range o.instances {
		tni.Close()
	}
}

// CreateProtocol returns a fresh Protocol Instance with an attached
// TreeNodeInstance
func (o *Overlay) CreateProtocol(t *Tree, name string) (ProtocolInstance, error) {
	tni := o.NewTreeNodeInstanceFromProtoName(t, name)
	pi, err := ProtocolInstantiate(ProtocolNameToID(name), tni)
	o.RegisterProtocolInstance(pi)
	go pi.Dispatch()
	return pi, err
}

// NewTreeNodeInstanceFromProtoName takes a protocol name and a tree and
// instantiate a TreeNodeInstance for this protocol.
func (o *Overlay) NewTreeNodeInstanceFromProtoName(t *Tree, name string) *TreeNodeInstance {
	return o.NewTreeNodeInstanceFromProtocol(t, t.Root, ProtocolNameToID(name))
}

// NewTreeNodeInstanceFromProtocol takes a tree and a treenode (normally the
// root) and and protocolID and returns a fresh TreeNodeInstance.
func (o *Overlay) NewTreeNodeInstanceFromProtocol(t *Tree, tn *TreeNode, protoID ProtocolID) *TreeNodeInstance {
	tok := &Token{
		TreeNodeID:   tn.Id,
		TreeID:       t.Id,
		EntityListID: t.EntityList.Id,
		ProtoID:      protoID,
		RoundID:      RoundID(uuid.NewV4()),
	}
	tni := o.newTreeNodeInstanceFromToken(tn, tok)
	o.RegisterTree(t)
	o.RegisterEntityList(t.EntityList)
	return tni
}
func (o *Overlay) NewTreeNodeInstanceFromService(t *Tree, tn *TreeNode, servID ServiceID) *TreeNodeInstance {
	tok := &Token{
		TreeNodeID:   tn.Id,
		TreeID:       t.Id,
		EntityListID: t.EntityList.Id,
		ServiceID:    servID,
		RoundID:      RoundID(uuid.NewV4()),
	}
	tni := o.newTreeNodeInstanceFromToken(tn, tok)
	o.RegisterTree(t)
	o.RegisterEntityList(t.EntityList)
	return tni
}

// newTreeNodeInstanceFromToken is to be called by the Overlay when it receives
// a message it does not have a treenodeinstance registered yet. The protocol is
// already running so we should *not* generate a new RoundID.
func (o *Overlay) newTreeNodeInstanceFromToken(tn *TreeNode, tok *Token) *TreeNodeInstance {
	tni := newTreeNodeInstance(o, tok, tn)
	o.instancesLock.Lock()
	defer o.instancesLock.Unlock()
	o.instances[tok.Id()] = tni
	dbg.Lvl4(o.host.workingAddress, "Registered NewTreeNodeInstance!")
	return tni
}

var ErrWrongTreeNodeInstance = errors.New("TreeNodeInstance associated with this ProtocolInstance is already registered")
var ErrProtocolRegistered = errors.New("A ProtocolInstance already has been registered using this TreeNodeInstance!")

func (o *Overlay) RegisterProtocolInstance(pi ProtocolInstance) error {
	o.instancesLock.Lock()
	defer o.instancesLock.Unlock()
	var tni *TreeNodeInstance
	var tok = pi.Token()
	var ok bool
	// if the TreeNodeInstance is already registered here
	if tni, ok = o.instances[tok.Id()]; !ok {
		return ErrWrongTreeNodeInstance
	}

	if tni.isBinded() {
		return ErrProtocolRegistered
	}

	tni.bind(pi)
	o.pis[tok.Id()] = pi
	dbg.Lvl4(o.host.workingAddress, "Registered ProtocolInstance !", fmt.Sprintf("%+v", tok))
	return nil
}

// TreeNodeCache is a cache that maps from token to treeNode. Since the mapping
// is not 1-1 (many Token can point to one TreeNode, but one token leads to one
// TreeNode), we have to do certain
// lookup, but that's better than searching the tree each time.
type TreeNodeCache map[TreeID]map[TreeNodeID]*TreeNode

// Returns a new TreeNodeCache
func NewTreeNodeCache() TreeNodeCache {
	m := make(map[TreeID]map[TreeNodeID]*TreeNode)
	return m
}

// Cache a TreeNode that relates to the Tree
// It will also cache the parent and children of the treenode since that's most
// likely what we are going to query.
func (tnc TreeNodeCache) Cache(tree *Tree, treeNode *TreeNode) {
	var mm map[TreeNodeID]*TreeNode
	var ok bool
	if mm, ok = tnc[tree.Id]; !ok {
		mm = make(map[TreeNodeID]*TreeNode)
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
	var mm map[TreeNodeID]*TreeNode
	var ok bool
	if tok == nil {
		return nil
	}
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
