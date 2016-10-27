package sda

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
)

// Overlay keeps all trees and entity-lists for a given Conode. It creates
// Nodes and ProtocolInstances upon request and dispatches the messages.
type Overlay struct {
	conode *Conode
	// mapping from Tree.Id to Tree
	trees    map[TreeID]*Tree
	treesMut sync.Mutex
	// mapping from Roster.id to Roster
	entityLists    map[RosterID]*Roster
	entityListLock sync.Mutex
	// cache for relating token(~Node) to TreeNode
	cache *TreeNodeCache

	// TreeNodeInstance part
	instances         map[TokenID]*TreeNodeInstance
	instancesInfo     map[TokenID]bool
	instancesLock     sync.Mutex
	protocolInstances map[TokenID]ProtocolInstance

	// treeMarshal that needs to be converted to Tree but host does not have the
	// entityList associated yet.
	// map from Roster.ID => trees that use this entity list
	pendingTreeMarshal map[RosterID][]*TreeMarshal
	// lock associated with pending TreeMarshal
	pendingTreeLock sync.Mutex

	// pendingSDAData are a list of message we received that does not correspond
	// to any local Tree or/and Roster. We first request theses so we can
	// instantiate properly protocolInstance that will use these SDAData msg.
	pendingSDAs []*ProtocolMsg
	// lock associated with pending SDAdata
	pendingSDAsLock sync.Mutex

	transmitMux sync.Mutex
}

// NewOverlay creates a new overlay-structure
func NewOverlay(c *Conode) *Overlay {
	o := &Overlay{
		conode:             c,
		trees:              make(map[TreeID]*Tree),
		entityLists:        make(map[RosterID]*Roster),
		cache:              NewTreeNodeCache(),
		instances:          make(map[TokenID]*TreeNodeInstance),
		instancesInfo:      make(map[TokenID]bool),
		protocolInstances:  make(map[TokenID]ProtocolInstance),
		pendingTreeMarshal: make(map[RosterID][]*TreeMarshal),
		pendingSDAs:        make([]*ProtocolMsg, 0),
	}
	// messages going to protocol instances
	c.RegisterProcessor(o,
		SDADataMessageID,       // protocol instance's messages
		RequestTreeMessageID,   // request a tree
		SendTreeMessageID,      // send a tree back to a request
		RequestRosterMessageID, // request a roster
		SendRosterMessageID)    // send a roster back to request
	return o
}

// Process implements the Processor interface so it process the messages that it
// wants.
func (o *Overlay) Process(data *network.Packet) {
	switch data.MsgType {
	case SDADataMessageID:
		sdaMsg := data.Msg.(ProtocolMsg)
		sdaMsg.ServerIdentity = data.ServerIdentity
		err := o.TransmitMsg(&sdaMsg)
		if err != nil {
			log.Error("ProcessSDAMessage returned:", err)
		}

	case RequestTreeMessageID:
		// A host has sent us a request to get a tree definition
		tid := data.Msg.(RequestTree).TreeID
		tree := o.Tree(tid)
		var err error
		if tree != nil {
			err = o.conode.Send(data.ServerIdentity, tree.MakeTreeMarshal())
		} else {
			// XXX Take care here for we must verify at the other side that
			// the tree is Nil. Should we think of a way of sending back an
			// "error" ?
			err = o.conode.Send(data.ServerIdentity, (&Tree{}).MakeTreeMarshal())
		}
		if err != nil {
			log.Error("Couldn't send tree:", err)
		}
	case SendTreeMessageID:
		// A Host has replied to our request of a tree
		tm := data.Msg.(TreeMarshal)
		if tm.TreeID == TreeID(uuid.Nil) {
			log.Error("Received an empty Tree")
			return
		}
		il := o.Roster(tm.RosterID)
		// The entity list does not exists, we should request that, too
		if il == nil {
			msg := &RequestRoster{tm.RosterID}
			if err := o.conode.Send(data.ServerIdentity, msg); err != nil {
				log.Error("Requesting Roster in SendTree failed", err)
			}

			// put the tree marshal into pending queue so when we receive the
			// entitylist we can create the real Tree.
			o.addPendingTreeMarshal(&tm)
			return
		}

		tree, err := tm.MakeTree(il)
		if err != nil {
			log.Error("Couldn't create tree:", err)
			return
		}
		log.Lvl4("Received new tree")
		o.RegisterTree(tree)
	case RequestRosterMessageID:
		// Some host requested an Roster
		id := data.Msg.(RequestRoster).RosterID
		el := o.Roster(id)
		var err error
		if el != nil {
			err = o.conode.Send(data.ServerIdentity, el)
		} else {
			log.Lvl2("Requested entityList that we don't have")
			err = o.conode.Send(data.ServerIdentity, &Roster{})
		}
		if err != nil {
			log.Error("Couldn't send empty entity list from host:",
				o.conode.ServerIdentity.String(),
				err)
			return
		}
	case SendRosterMessageID:
		// Host replied to our request of entitylist
		il := data.Msg.(Roster)
		if il.ID == RosterID(uuid.Nil) {
			log.Lvl2("Received an empty Roster")
		} else {
			o.RegisterRoster(&il)
			// Check if some trees can be constructed from this entitylist
			o.checkPendingTreeMarshal(&il)
		}
		log.Lvl4("Received new entityList")
	}
}

// TransmitMsg takes a message received from the host and treats it. It might
// - ask for the identityList
// - ask for the Tree
// - create a new protocolInstance
// - pass it to a given protocolInstance
func (o *Overlay) TransmitMsg(sdaMsg *ProtocolMsg) error {
	tree := o.Tree(sdaMsg.To.TreeID)
	if tree == nil {
		return o.requestTree(sdaMsg.ServerIdentity, sdaMsg)
	}

	o.transmitMux.Lock()
	defer o.transmitMux.Unlock()
	// TreeNodeInstance
	var pi ProtocolInstance
	o.instancesLock.Lock()
	pi, ok := o.protocolInstances[sdaMsg.To.ID()]
	done := o.instancesInfo[sdaMsg.To.ID()]
	o.instancesLock.Unlock()
	if done {
		log.Error("Message for TreeNodeInstance that is already finished")
		return nil
	}
	// if the TreeNodeInstance is not there, creates it
	if !ok {
		log.Lvlf4("Creating TreeNodeInstance at %s %x", o.conode.ServerIdentity, sdaMsg.To.ID())
		tn, err := o.TreeNodeFromToken(sdaMsg.To)
		if err != nil {
			return errors.New("No TreeNode defined in this tree here")
		}
		tni := o.newTreeNodeInstanceFromToken(tn, sdaMsg.To)
		// see if we know the Service Recipient
		s, ok := o.conode.serviceManager.serviceByID(sdaMsg.To.ServiceID)

		// no servies defined => check if there is a protocol that can be
		// created
		if !ok {
			pi, err = o.conode.ProtocolInstantiate(sdaMsg.To.ProtoID, tni)
			if err != nil {
				return err
			}
			go pi.Dispatch()

			/// use the Services to instantiate it
		} else {
			// request the PI from the Service and binds the two
			pi, err = s.NewProtocol(tni, &sdaMsg.Config)
			if err != nil {
				return err
			}
			if pi == nil {
				return nil
			}
			go pi.Dispatch()
		}
		if err := o.RegisterProtocolInstance(pi); err != nil {
			return errors.New("Error Binding TreeNodeInstance and ProtocolInstance: " +
				err.Error())
		}
		log.Lvl4(o.conode.Address(), "Overlay created new ProtocolInstace msg => ",
			fmt.Sprintf("%+v", sdaMsg.To))

	}

	// TODO Check if TreeNodeInstance is already Done
	pi.ProcessProtocolMsg(sdaMsg)
	return nil
}

// sendSDAData marshals the inner msg and then sends a Data msg
// to the appropriate entity
func (o *Overlay) sendSDAData(si *network.ServerIdentity, sdaMsg *ProtocolMsg) error {
	b, err := network.MarshalRegisteredType(sdaMsg.Msg)
	if err != nil {
		return fmt.Errorf("Error marshaling message: %s (msg = %+v)", err.Error(), sdaMsg.Msg)
	}
	sdaMsg.MsgSlice = b
	sdaMsg.MsgType = network.TypeFromData(sdaMsg.Msg)
	// put to nil so protobuf won't encode it and there won't be any error on the
	// other side (because it doesn't know how to decode it)
	sdaMsg.Msg = nil
	log.Lvl4(o.conode.Address(), "Sending to", si.Address)
	return o.conode.Send(si, sdaMsg)
}

// addPendingTreeMarshal adds a treeMarshal to the list.
// This list is checked each time we receive a new Roster
// so trees using this Roster can be constructed.
func (o *Overlay) addPendingTreeMarshal(tm *TreeMarshal) {
	o.pendingTreeLock.Lock()
	var sl []*TreeMarshal
	var ok bool
	// initiate the slice before adding
	if sl, ok = o.pendingTreeMarshal[tm.RosterID]; !ok {
		sl = make([]*TreeMarshal, 0)
	}
	sl = append(sl, tm)
	o.pendingTreeMarshal[tm.RosterID] = sl
	o.pendingTreeLock.Unlock()
}

// checkPendingMessages is called each time we receive a new tree if there are some SDA
// messages using this tree. If there are, we can make an instance of a protocolinstance
// and give it the message!.
// NOTE: put that as a go routine so the rest of the processing messages are not
// slowed down, if there are many pending sda message at once (i.e. start many new
// protocols at same time)
func (o *Overlay) checkPendingMessages(t *Tree) {
	go func() {
		o.pendingSDAsLock.Lock()
		var newPending []*ProtocolMsg
		for _, msg := range o.pendingSDAs {
			if t.ID.Equals(msg.To.TreeID) {
				// if this message references t, instantiate it and go
				err := o.TransmitMsg(msg)
				if err != nil {
					log.Error("TransmitMsg failed:", err)
					continue
				}
			} else {
				newPending = append(newPending, msg)
			}
		}
		o.pendingSDAs = newPending
		o.pendingSDAsLock.Unlock()
	}()
}

// checkPendingTreeMarshal is called each time we add a new Roster to the
// system. It checks if some treeMarshal use this entityList so they can be
// converted to Tree.
func (o *Overlay) checkPendingTreeMarshal(el *Roster) {
	o.pendingTreeLock.Lock()
	sl, ok := o.pendingTreeMarshal[el.ID]
	if !ok {
		// no tree for this entitty list
		return
	}
	for _, tm := range sl {
		tree, err := tm.MakeTree(el)
		if err != nil {
			log.Error("Tree from Roster failed")
			continue
		}
		// add the tree into our "database"
		o.RegisterTree(tree)
	}
	o.pendingTreeLock.Unlock()
}

// requestTree will ask for the tree the sdadata is related to.
// it will put the message inside the pending list of sda message waiting to
// have their trees.
func (o *Overlay) requestTree(si *network.ServerIdentity, sdaMsg *ProtocolMsg) error {
	o.pendingSDAsLock.Lock()
	o.pendingSDAs = append(o.pendingSDAs, sdaMsg)
	o.pendingSDAsLock.Unlock()

	treeRequest := &RequestTree{sdaMsg.To.TreeID}

	return o.conode.Send(si, treeRequest)
}

// RegisterTree takes a tree and puts it in the map
func (o *Overlay) RegisterTree(t *Tree) {
	o.treesMut.Lock()
	o.trees[t.ID] = t
	o.treesMut.Unlock()
	o.checkPendingMessages(t)
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

// RegisterRoster puts an entityList in the map
func (o *Overlay) RegisterRoster(el *Roster) {
	o.entityListLock.Lock()
	defer o.entityListLock.Unlock()
	o.entityLists[el.ID] = el
}

// RosterFromToken returns the entitylist corresponding to a token
func (o *Overlay) RosterFromToken(tok *Token) *Roster {
	return o.entityLists[tok.RosterID]
}

// Roster returns the entityList given by RosterID
func (o *Overlay) Roster(elid RosterID) *Roster {
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
func (o *Overlay) SendToTreeNode(from *Token, to *TreeNode, msg network.Body) error {
	sda := &ProtocolMsg{
		Msg:  msg,
		From: from,
		To:   from.ChangeTreeNodeID(to.ID),
	}
	log.Lvl4(o.conode.Address(), "Sending to entity", to.ServerIdentity.Address)
	return o.sendSDAData(to.ServerIdentity, sda)
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
	tni, ok := o.instances[tok.ID()]
	if !ok {
		log.Lvlf2("Node %x already gone", tok.ID())
		return
	}
	log.Lvl4("Closing node", tok.ID())
	err := tni.Close()
	if err != nil {
		log.Error("Error while closing node:", err)
	}
	delete(o.instances, tok.ID())
	// mark it done !
	o.instancesInfo[tok.ID()] = true
}

func (o *Overlay) suite() abstract.Suite {
	return o.conode.Suite()
}

// Close calls all nodes, deletes them from the list and closes them
func (o *Overlay) Close() {
	o.instancesLock.Lock()
	defer o.instancesLock.Unlock()
	for _, tni := range o.instances {
		log.Lvl4(o.conode.Address(), "Closing TNI", tni.TokenID())
		o.nodeDelete(tni.Token())
	}
}

// CreateProtocolSDA returns a fresh Protocol Instance with an attached
// TreeNodeInstance. This protocol won't be handled by the service, but
// only by the SDA.
func (o *Overlay) CreateProtocolSDA(name string, t *Tree) (ProtocolInstance, error) {
	return o.CreateProtocolService(name, t, ServiceID(uuid.Nil))
}

// CreateProtocolService adds the service-id to the token so the protocol will
// be picked up by the correct service and handled by its NewProtocol method.
func (o *Overlay) CreateProtocolService(name string, t *Tree, sid ServiceID) (ProtocolInstance, error) {
	tni := o.NewTreeNodeInstanceFromService(t, t.Root, ProtocolNameToID(name), sid)
	pi, err := o.conode.ProtocolInstantiate(tni.token.ProtoID, tni)
	if err != nil {
		return nil, err
	}
	if err = o.RegisterProtocolInstance(pi); err != nil {
		return nil, err
	}
	go pi.Dispatch()
	return pi, err
}

// StartProtocol will create and start a P.I.
func (o *Overlay) StartProtocol(t *Tree, name string) (ProtocolInstance, error) {
	pi, err := o.CreateProtocolSDA(name, t)
	if err != nil {
		return nil, err
	}
	go func() {
		err := pi.Start()
		if err != nil {
			log.Error("Error while starting:", err)
		}
	}()
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
		TreeNodeID: tn.ID,
		TreeID:     t.ID,
		RosterID:   t.Roster.ID,
		ProtoID:    protoID,
		RoundID:    RoundID(uuid.NewV4()),
	}
	tni := o.newTreeNodeInstanceFromToken(tn, tok)
	o.RegisterTree(t)
	o.RegisterRoster(t.Roster)
	return tni
}

// NewTreeNodeInstanceFromService takes a tree, a TreeNode and a service ID and
// returns a TNI.
func (o *Overlay) NewTreeNodeInstanceFromService(t *Tree, tn *TreeNode, protoID ProtocolID, servID ServiceID) *TreeNodeInstance {
	tok := &Token{
		TreeNodeID: tn.ID,
		TreeID:     t.ID,
		RosterID:   t.Roster.ID,
		ProtoID:    protoID,
		ServiceID:  servID,
		RoundID:    RoundID(uuid.NewV4()),
	}
	tni := o.newTreeNodeInstanceFromToken(tn, tok)
	o.RegisterTree(t)
	o.RegisterRoster(t.Roster)
	return tni
}

// ServerIdentity Returns the entity of the Host
func (o *Overlay) ServerIdentity() *network.ServerIdentity {
	return o.conode.ServerIdentity
}

// newTreeNodeInstanceFromToken is to be called by the Overlay when it receives
// a message it does not have a treenodeinstance registered yet. The protocol is
// already running so we should *not* generate a new RoundID.
func (o *Overlay) newTreeNodeInstanceFromToken(tn *TreeNode, tok *Token) *TreeNodeInstance {
	tni := newTreeNodeInstance(o, tok, tn)
	o.instancesLock.Lock()
	defer o.instancesLock.Unlock()
	o.instances[tok.ID()] = tni
	return tni
}

// ErrWrongTreeNodeInstance is returned when you already binded a TNI with a PI.
var ErrWrongTreeNodeInstance = errors.New("This TreeNodeInstance doesn't exist")

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
	// if the TreeNodeInstance doesn't exist
	if tni, ok = o.instances[tok.ID()]; !ok {
		return ErrWrongTreeNodeInstance
	}

	if tni.isBound() {
		return ErrProtocolRegistered
	}

	tni.bind(pi)
	o.protocolInstances[tok.ID()] = pi
	log.Lvlf4("%s registered ProtocolInstance %x", o.conode.Address(), tok.ID())
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

// NewTreeNodeCache Returns a new TreeNodeCache
func NewTreeNodeCache() *TreeNodeCache {
	return &TreeNodeCache{
		Entries: make(map[TreeID]map[TreeNodeID]*TreeNode),
	}
}

// Cache a TreeNode that relates to the Tree
// It will also cache the parent and children of the treenode since that's most
// likely what we are going to query.
func (tnc *TreeNodeCache) Cache(tree *Tree, treeNode *TreeNode) {
	tnc.Lock()
	defer tnc.Unlock()
	mm, ok := tnc.Entries[tree.ID]
	if !ok {
		mm = make(map[TreeNodeID]*TreeNode)
	}
	// add treenode
	mm[treeNode.ID] = treeNode
	// add parent if not root
	if treeNode.Parent != nil {
		mm[treeNode.Parent.ID] = treeNode.Parent
	}
	// add children
	for _, c := range treeNode.Children {
		mm[c.ID] = c
	}
	// add cache
	tnc.Entries[tree.ID] = mm
}

// GetFromToken returns the TreeNode that the token is pointing at, or
// nil if there is none for this token.
func (tnc *TreeNodeCache) GetFromToken(tok *Token) *TreeNode {
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
