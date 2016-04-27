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
	// mapping from Tree.Id to Tree
	trees    map[TreeID]*Tree
	treesMut sync.Mutex
	// mapping from EntityList.id to EntityList
	entityLists    map[EntityListID]*EntityList
	entityListLock sync.Mutex
	// cache for relating token(~Node) to TreeNode
	cache *TreeNodeCache

	// TreeNodeInstance part
	instances         map[TokenID]*TreeNodeInstance
	instancesInfo     map[TokenID]bool
	instancesLock     sync.Mutex
	protocolInstances map[TokenID]ProtocolInstance

	transmitMux sync.Mutex
}

// NewOverlay creates a new overlay-structure
func NewOverlay(h *Host) *Overlay {
	return &Overlay{
		host:              h,
		trees:             make(map[TreeID]*Tree),
		entityLists:       make(map[EntityListID]*EntityList),
		cache:             NewTreeNodeCache(),
		instances:         make(map[TokenID]*TreeNodeInstance),
		instancesInfo:     make(map[TokenID]bool),
		protocolInstances: make(map[TokenID]ProtocolInstance),
		transmitMux:       sync.Mutex{},
	}
}

// TransmitMsg takes a message received from the host and treats it. It might
// - ask for the identityList
// - ask for the Tree
// - create a new protocolInstance
// - pass it to a given protocolInstance
func (o *Overlay) TransmitMsg(sdaMsg *Data) error {
	o.transmitMux.Lock()
	defer o.transmitMux.Unlock()
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
	pi, ok := o.protocolInstances[sdaMsg.To.Id()]
	done := o.instancesInfo[sdaMsg.To.Id()]
	o.instancesLock.Unlock()
	if done {
		dbg.Lvl3("Message for TreeNodeInstance that is already finished")
		return nil
	}
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
			// request the PI from the Service and bind the two
			pi, err = s.NewProtocol(tni, &sdaMsg.Config)
			if err != nil {
				return err
			}
		}
		if err := o.RegisterProtocolInstance(pi); err != nil {
			return errors.New("Error Binding TreeNodeInstance and ProtocolInstance: " +
				err.Error())
		}
		dbg.Lvl4(o.host.workingAddress, "Overlay created new ProtocolInstace msg => ",
			fmt.Sprintf("%+v", sdaMsg.To))

	}
	// TODO Check if TreeNodeInstance is already Done
	pi.DispatchMsg(sdaMsg)

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
	o.treesMut.Lock()
	defer o.treesMut.Unlock()
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

// nodeDone is called by node to signify that its work is finished and its
// ressources can be released
func (o *Overlay) nodeDone(tok *Token) {
	o.instancesLock.Lock()
	defer o.instancesLock.Unlock()
	o.nodeDelete(tok)
}

// nodeDelete needs to be separated from nodeDone, as it is also called from
// Close, but due to locking-issues here we don't lock.
func (o *Overlay) nodeDelete(tok *Token) {
	tni, ok := o.instances[tok.Id()]
	if !ok {
		dbg.Lvl2("Node", tok.Id(), "already gone")
		return
	}
	dbg.Lvl4("Closing node", tok.Id())
	err := tni.Close()
	if err != nil {
		dbg.Error("Error while closing node:", err)
	}
	delete(o.instances, tok.Id())
	// mark it done !
	o.instancesInfo[tok.Id()] = true
}

func (o *Overlay) suite() abstract.Suite {
	return o.host.Suite()
}

// Close calls all nodes, deletes them from the list and closes them
func (o *Overlay) Close() {
	o.instancesLock.Lock()
	defer o.instancesLock.Unlock()
	for _, tni := range o.instances {
		dbg.Lvl4(o.host.workingAddress, "Closing TNI", tni.TokenID())
		o.nodeDelete(tni.Token())
	}
}

// CreateProtocol returns a fresh Protocol Instance with an attached
// TreeNodeInstance
func (o *Overlay) CreateProtocol(t *Tree, name string) (ProtocolInstance, error) {
	tni := o.NewTreeNodeInstanceFromProtoName(t, name)
	pi, err := ProtocolInstantiate(ProtocolNameToID(name), tni)
	if err != nil {
		return nil, err
	}
	o.RegisterProtocolInstance(pi)
	go pi.Dispatch()
	return pi, err
}

// StartProtocol will create and start a P.I.
func (o *Overlay) StartProtocol(t *Tree, name string) (ProtocolInstance, error) {
	pi, err := o.CreateProtocol(t, name)
	if err != nil {
		return nil, err
	}
	go pi.Start()
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

// NewTreeNodeInstanceFromService takes a tree, a TreeNode and a service ID and
// returns a TNI.
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

// ErrWrongTreeNodeInstance is returned when you already binded a TNI with a PI.
var ErrWrongTreeNodeInstance = errors.New("TreeNodeInstance associated with this ProtocolInstance is already registered")

// ErrProtocolRegistered is when the protocolinstance is already registered to
// the overlay
var ErrProtocolRegistered = errors.New("A ProtocolInstance already has been registered using this TreeNodeInstance!")

// RegisterProtocolInstance takes a PI and stores it for dispatching the message
// to it.
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

	if tni.isBound() {
		return ErrProtocolRegistered
	}

	tni.bind(pi)
	o.protocolInstances[tok.Id()] = pi
	dbg.Lvl4(o.host.workingAddress, "Registered ProtocolInstance !", fmt.Sprintf("%+v", tok))
	return nil
}

// TreeNodeCache is a cache that maps from token to treeNode. Since the mapping
// is not 1-1 (many Token can point to one TreeNode, but one token leads to one
// TreeNode), we have to do certain
// lookup, but that's better than searching the tree each time.
type TreeNodeCache struct {
	Entries map[TreeID]map[TreeNodeID]*TreeNode
	sync.Mutex
}

// Returns a new TreeNodeCache
func NewTreeNodeCache() *TreeNodeCache {
	return &TreeNodeCache{
		Entries: make(map[TreeID]map[TreeNodeID]*TreeNode),
	}
}

// Cache a TreeNode that relates to the Tree
// It will also cache the parent and children of the treenode since that's most
// likely what we are going to query.
func (tnc TreeNodeCache) Cache(tree *Tree, treeNode *TreeNode) {
	tnc.Lock()
	defer tnc.Unlock()
	mm, ok := tnc.Entries[tree.Id]
	if !ok {
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
	tnc.Entries[tree.Id] = mm
}

// GetFromToken returns the TreeNode that the token is pointing at, or
// nil if there is none for this token.
func (tnc TreeNodeCache) GetFromToken(tok *Token) *TreeNode {
	tnc.Lock()
	defer tnc.Unlock()
	if tok == nil {
		return nil
	}
	mm, ok := tnc.Entries[tok.TreeID]
	if !ok {
		// no tree cached for this token :...
		return nil
	}
	tn, ok := mm[tok.TreeNodeID]
	if !ok {
		// no treeNode cached for this token...
		// XXX Should we search the tree ? Then we need to keep reference to the
		// tree ...
		return nil
	}
	return tn
}
