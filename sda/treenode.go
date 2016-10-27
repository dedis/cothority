package sda

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"strings"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
)

// TreeNodeInstance represents a protocol-instance in a given TreeNode. It embeds an
// Overlay where all the tree-structures are stored.
type TreeNodeInstance struct {
	overlay *Overlay
	token   *Token
	// cache for the TreeNode this Node is representing
	treeNode *TreeNode
	// cached list of all TreeNodes
	treeNodeList []*TreeNode
	// mutex to synchronise creation of treeNodeList
	mtx sync.Mutex

	// channels holds all channels available for the different message-types
	channels map[network.PacketTypeID]interface{}
	// registered handler-functions for that protocol
	handlers map[network.PacketTypeID]interface{}
	// flags for messages - only one channel/handler possible
	messageTypeFlags map[network.PacketTypeID]uint32
	// The protocolInstance belonging to that node
	instance ProtocolInstance
	// aggregate messages in order to dispatch them at once in the protocol
	// instance
	msgQueue map[network.PacketTypeID][]*ProtocolMsg
	// done callback
	onDoneCallback func() bool
	// queue holding msgs
	msgDispatchQueue []*ProtocolMsg
	// locking for msgqueue
	msgDispatchQueueMutex sync.Mutex
	// kicking off new message
	msgDispatchQueueWait chan bool
	// whether this node is closing
	closing bool
}

// aggregateMessages (if set) tells to aggregate messages from all children
// before sending to the (parent) Node
// https://golang.org/ref/spec#Iota
const (
	AggregateMessages = 1 << iota
)

// MsgHandler is called upon reception of a certain message-type
type MsgHandler func([]*interface{})

// NewNode creates a new node
func newTreeNodeInstance(o *Overlay, tok *Token, tn *TreeNode) *TreeNodeInstance {
	n := &TreeNodeInstance{overlay: o,
		token:                tok,
		channels:             make(map[network.PacketTypeID]interface{}),
		handlers:             make(map[network.PacketTypeID]interface{}),
		messageTypeFlags:     make(map[network.PacketTypeID]uint32),
		msgQueue:             make(map[network.PacketTypeID][]*ProtocolMsg),
		treeNode:             tn,
		msgDispatchQueue:     make([]*ProtocolMsg, 0, 1),
		msgDispatchQueueWait: make(chan bool, 1),
	}
	go n.dispatchMsgReader()
	return n
}

// TreeNode gets the treeNode of this node. If there is no TreeNode for the
// Token of this node, the function will return nil
func (n *TreeNodeInstance) TreeNode() *TreeNode {
	return n.treeNode
}

// ServerIdentity returns our entity
func (n *TreeNodeInstance) ServerIdentity() *network.ServerIdentity {
	return n.treeNode.ServerIdentity
}

// Parent returns the parent-TreeNode of ourselves
func (n *TreeNodeInstance) Parent() *TreeNode {
	return n.treeNode.Parent
}

// Children returns the children of ourselves
func (n *TreeNodeInstance) Children() []*TreeNode {
	return n.treeNode.Children
}

// Root returns the root-node of that tree
func (n *TreeNodeInstance) Root() *TreeNode {
	return n.Tree().Root
}

// IsRoot returns whether whether we are at the top of the tree
func (n *TreeNodeInstance) IsRoot() bool {
	return n.treeNode.Parent == nil
}

// IsLeaf returns whether whether we are at the bottom of the tree
func (n *TreeNodeInstance) IsLeaf() bool {
	return len(n.treeNode.Children) == 0
}

// SendTo sends to a given node
func (n *TreeNodeInstance) SendTo(to *TreeNode, msg interface{}) error {
	if to == nil {
		return errors.New("Sent to a nil TreeNode")
	}
	//LUDOVIC BARMAN : ADD UDP HERE
	return n.overlay.SendToTreeNode(n.token, to, msg)
}

// Tree returns the tree of that node
func (n *TreeNodeInstance) Tree() *Tree {
	return n.overlay.TreeFromToken(n.token)
}

// Roster returns the entity-list
func (n *TreeNodeInstance) Roster() *Roster {
	return n.Tree().Roster
}

// Suite can be used to get the current abstract.Suite (currently hardcoded into
// the network library).
func (n *TreeNodeInstance) Suite() abstract.Suite {
	return n.overlay.suite()
}

// RegisterChannel takes a channel with a struct that contains two
// elements: a TreeNode and a message. It will send every message that are the
// same type to this channel.
// This function handles also
// - registration of the message-type
// - aggregation or not of messages: if you give a channel of slices, the
//   messages will be aggregated, else they will come one-by-one
func (n *TreeNodeInstance) RegisterChannel(c interface{}) error {
	flags := uint32(0)
	cr := reflect.TypeOf(c)
	if cr.Kind() == reflect.Ptr {
		val := reflect.ValueOf(c).Elem()
		val.Set(reflect.MakeChan(val.Type(), 100))
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
	if cr.Elem().NumField() != 2 {
		return errors.New("Input is not channel of structure with 2 elements")
	}
	if cr.Elem().Field(0).Type != reflect.TypeOf(&TreeNode{}) {
		return errors.New("Input-channel doesn't have TreeNode as element")
	}
	// Automatic registration of the message to the network library.
	typ := network.RegisterPacketUUID(network.RTypeToPacketTypeID(
		cr.Elem().Field(1).Type),
		cr.Elem().Field(1).Type)
	n.channels[typ] = c
	//typ := network.RTypeToUUID(cr.Elem().Field(1).Type) n.channels[typ] = c
	n.messageTypeFlags[typ] = flags
	log.Lvl4("Registered channel", typ, "with flags", flags)
	return nil
}

// RegisterChannels registers a list of given channels by calling RegisterChannel above
func (n *TreeNodeInstance) RegisterChannels(channels ...interface{}) error {
	for _, ch := range channels {
		if err := n.RegisterChannel(ch); err != nil {
			return fmt.Errorf("Error, could not register channel %T: %s",
				ch, err.Error())
		}
	}
	return nil
}

// RegisterHandler takes a function which takes a struct as argument that contains two
// elements: a TreeNode and a message. It will send every message that are the
// same type to this channel.
// This function handles also
// - registration of the message-type
// - aggregation or not of messages: if you give a channel of slices, the
//   messages will be aggregated, else they will come one-by-one
func (n *TreeNodeInstance) RegisterHandler(c interface{}) error {
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
	if cr.NumField() != 2 {
		return errors.New("Input is not channel of structure with 2 elements")
	}
	if cr.Field(0).Type != reflect.TypeOf(&TreeNode{}) {
		return errors.New("Input-channel doesn't have TreeNode as element")
	}
	// Automatic registration of the message to the network library.
	typ := network.RegisterPacketUUID(network.RTypeToPacketTypeID(
		cr.Field(1).Type),
		cr.Field(1).Type)
	//typ := network.RTypeToUUID(cr.Elem().Field(1).Type)
	n.handlers[typ] = c
	n.messageTypeFlags[typ] = flags
	log.Lvl3("Registered handler", typ, "with flags", flags)
	return nil
}

// RegisterHandlers registers a list of given handlers by calling RegisterHandler above
func (n *TreeNodeInstance) RegisterHandlers(handlers ...interface{}) error {
	for _, h := range handlers {
		if err := n.RegisterHandler(h); err != nil {
			return fmt.Errorf("Error, could not register handler %T: %s",
				h, err.Error())
		}
	}
	return nil
}

// ProtocolInstance returns the instance of the running protocol
func (n *TreeNodeInstance) ProtocolInstance() ProtocolInstance {
	return n.instance
}

// Dispatch - the standard dispatching function is empty
func (n *TreeNodeInstance) Dispatch() error {
	return nil
}

// Shutdown - standard Shutdown implementation. Define your own
// in your protocol (if necessary)
func (n *TreeNodeInstance) Shutdown() error {
	return nil
}

// Close shuts down the go-routine and calls the protocolInstance-shutdown
func (n *TreeNodeInstance) Close() error {
	log.Lvl3("Closing node", n.Info())
	n.msgDispatchQueueMutex.Lock()
	n.closing = true
	if len(n.msgDispatchQueueWait) == 0 {
		n.msgDispatchQueueWait <- true
	}
	n.msgDispatchQueueMutex.Unlock()
	return n.ProtocolInstance().Shutdown()
}

// ProtocolName will return the string representing that protocol
func (n *TreeNodeInstance) ProtocolName() string {
	return n.overlay.conode.protocols.ProtocolIDToName(n.token.ProtoID)
}

func (n *TreeNodeInstance) dispatchHandler(msgSlice []*ProtocolMsg) error {
	mt := msgSlice[0].MsgType
	to := reflect.TypeOf(n.handlers[mt]).In(0)
	f := reflect.ValueOf(n.handlers[mt])
	if n.HasFlag(mt, AggregateMessages) {
		msgs := reflect.MakeSlice(to, len(msgSlice), len(msgSlice))
		for i, msg := range msgSlice {
			msgs.Index(i).Set(n.reflectCreate(to.Elem(), msg))
		}
		log.Lvl4("Dispatching aggregation to", n.ServerIdentity().Address)
		f.Call([]reflect.Value{msgs})
	} else {
		for _, msg := range msgSlice {
			log.Lvl4("Dispatching to", n.ServerIdentity().Address)
			m := n.reflectCreate(to, msg)
			f.Call([]reflect.Value{m})
		}
	}
	log.Lvlf4("%s Done with handler for %s", n.Name(), f.Type())
	return nil
}

func (n *TreeNodeInstance) reflectCreate(t reflect.Type, msg *ProtocolMsg) reflect.Value {
	m := reflect.Indirect(reflect.New(t))
	tn := n.Tree().Search(msg.From.TreeNodeID)
	if tn != nil {
		m.Field(0).Set(reflect.ValueOf(tn))
		m.Field(1).Set(reflect.Indirect(reflect.ValueOf(msg.Msg)))
	}
	return m
}

// DispatchChannel takes a message and sends it to a channel
func (n *TreeNodeInstance) DispatchChannel(msgSlice []*ProtocolMsg) error {
	mt := msgSlice[0].MsgType
	to := reflect.TypeOf(n.channels[mt])
	if n.HasFlag(mt, AggregateMessages) {
		log.Lvl4("Received aggregated message of type:", mt)
		to = to.Elem()
		out := reflect.MakeSlice(to, len(msgSlice), len(msgSlice))
		for i, msg := range msgSlice {
			log.Lvl4("Dispatching aggregated to", to)
			m := n.reflectCreate(to.Elem(), msg)
			log.Lvl4("Adding msg", m, "to", n.ServerIdentity().Address)
			out.Index(i).Set(m)
		}
		reflect.ValueOf(n.channels[mt]).Send(out)
	} else {
		for _, msg := range msgSlice {
			out := n.channels[mt]
			m := n.reflectCreate(to.Elem(), msg)
			log.Lvl4(n.Name(), "Dispatching msg type", mt, " to", to, " :", m.Field(1).Interface())
			reflect.ValueOf(out).Send(m)
		}
	}
	return nil
}

// ProcessProtocolMsg takes a message and puts it into a queue for later processing.
// This allows a protocol to have a backlog of messages.
func (n *TreeNodeInstance) ProcessProtocolMsg(msg *ProtocolMsg) {
	log.Lvl4(n.Info(), "Received message")
	n.msgDispatchQueueMutex.Lock()
	n.msgDispatchQueue = append(n.msgDispatchQueue, msg)
	log.Lvl4(n.Info(), "DispatchQueue-length is", len(n.msgDispatchQueue))
	if len(n.msgDispatchQueue) == 1 && len(n.msgDispatchQueueWait) == 0 {
		n.msgDispatchQueueWait <- true
	}
	n.msgDispatchQueueMutex.Unlock()
}

func (n *TreeNodeInstance) dispatchMsgReader() {
	for {
		n.msgDispatchQueueMutex.Lock()
		if n.closing == true {
			log.Lvl3("Closing reader")
			n.msgDispatchQueueMutex.Unlock()
			return
		}
		if len(n.msgDispatchQueue) > 0 {
			log.Lvl4(n.Info(), "Read message and dispatching it",
				len(n.msgDispatchQueue))
			msg := n.msgDispatchQueue[0]
			n.msgDispatchQueue = n.msgDispatchQueue[1:]
			n.msgDispatchQueueMutex.Unlock()
			err := n.dispatchMsgToProtocol(msg)
			if err != nil {
				log.Error("Error while dispatching message:", err)
			}
		} else {
			n.msgDispatchQueueMutex.Unlock()
			log.Lvl4(n.Info(), "Waiting for message")
			<-n.msgDispatchQueueWait
		}
	}
}

// dispatchMsgToProtocol will dispatch this sda.Data to the right instance
func (n *TreeNodeInstance) dispatchMsgToProtocol(sdaMsg *ProtocolMsg) error {
	// Decode the inner message here. In older versions, it was decoded before,
	// but first there is no use to do it before, and then every protocols had
	// to manually registers their messages. Since it is done automatically by
	// the Node, decoding should also be done by the node.
	var err error
	t, msg, err := network.UnmarshalRegisteredType(sdaMsg.MsgSlice, network.DefaultConstructors(n.Suite()))
	if err != nil {
		log.Error(n.ServerIdentity(), "Error while unmarshalling inner message of SDAData", sdaMsg.MsgType, ":", err)
	}
	// Put the msg into SDAData
	sdaMsg.MsgType = t
	sdaMsg.Msg = msg

	// if message comes from parent, dispatch directly
	// if messages come from children we must aggregate them
	// if we still need to wait for additional messages, we return
	msgType, msgs, done := n.aggregate(sdaMsg)
	if !done {
		log.Lvl3(n.Name(), "Not done aggregating children msgs")
		return nil
	}
	log.Lvlf5("%s->%s: Message is: %+v", n.Name(), sdaMsg.Msg)

	switch {
	case n.channels[msgType] != nil:
		log.Lvl4(n.Name(), "Dispatching to channel")
		err = n.DispatchChannel(msgs)
	case n.handlers[msgType] != nil:
		log.Lvl4("Dispatching to handler", n.ServerIdentity().Address)
		err = n.dispatchHandler(msgs)
	default:
		return errors.New("This message-type is not handled by this protocol")
	}
	return err
}

// SetFlag makes sure a given flag is set
func (n *TreeNodeInstance) SetFlag(mt network.PacketTypeID, f uint32) {
	n.messageTypeFlags[mt] |= f
}

// ClearFlag makes sure a given flag is removed
func (n *TreeNodeInstance) ClearFlag(mt network.PacketTypeID, f uint32) {
	n.messageTypeFlags[mt] &^= f
}

// HasFlag returns true if the given flag is set
func (n *TreeNodeInstance) HasFlag(mt network.PacketTypeID, f uint32) bool {
	return n.messageTypeFlags[mt]&f != 0
}

// aggregate store the message for a protocol instance such that a protocol
// instances will get all its children messages at once.
// node is the node the host is representing in this Tree, and sda is the
// message being analyzed.
func (n *TreeNodeInstance) aggregate(sdaMsg *ProtocolMsg) (network.PacketTypeID, []*ProtocolMsg, bool) {
	mt := sdaMsg.MsgType
	fromParent := !n.IsRoot() && sdaMsg.From.TreeNodeID.Equal(n.Parent().ID)
	if fromParent || !n.HasFlag(mt, AggregateMessages) {
		return mt, []*ProtocolMsg{sdaMsg}, true
	}
	// store the msg according to its type
	if _, ok := n.msgQueue[mt]; !ok {
		n.msgQueue[mt] = make([]*ProtocolMsg, 0)
	}
	msgs := append(n.msgQueue[mt], sdaMsg)
	n.msgQueue[mt] = msgs
	log.Lvl4(n.ServerIdentity().Address, "received", len(msgs), "of", len(n.Children()), "messages")

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

// StartProtocol calls the Start() on the underlying protocol which in turn will
// initiate the first message to its children
func (n *TreeNodeInstance) StartProtocol() error {
	return n.instance.Start()
}

// Done calls onDoneCallback if available and only finishes when the return-
// value is true.
func (n *TreeNodeInstance) Done() {
	if n.onDoneCallback != nil {
		ok := n.onDoneCallback()
		if !ok {
			return
		}
	}
	log.Lvl3(n.Info(), "has finished. Deleting its resources")
	n.overlay.nodeDone(n.token)
}

// OnDoneCallback should be called if we want to control the Done() of the node.
// It is used by protocols that uses others protocols inside and that want to
// control when the final Done() should be called.
// the function should return true if the real Done() has to be called otherwise
// false.
func (n *TreeNodeInstance) OnDoneCallback(fn func() bool) {
	n.onDoneCallback = fn
}

// Private returns the private key of the entity
func (n *TreeNodeInstance) Private() abstract.Scalar {
	return n.Host().private
}

// Public returns the public key of the entity
func (n *TreeNodeInstance) Public() abstract.Point {
	return n.ServerIdentity().Public
}

// CloseHost closes the underlying sda.Host (which closes the overlay
// and sends Shutdown to all protocol instances)
func (n *TreeNodeInstance) CloseHost() error {
	return n.Host().Close()
}

// Name returns a human readable name of this Node (IP address).
func (n *TreeNodeInstance) Name() string {
	return n.ServerIdentity().Address.String()
}

// Info returns a human readable representation name of this Node
// (IP address and TokenID).
func (n *TreeNodeInstance) Info() string {
	tid := n.TokenID()
	return fmt.Sprintf("%s (%s)", n.ServerIdentity().Address, tid.String())
}

// TokenID returns the TokenID of the given node (to uniquely identify it)
func (n *TreeNodeInstance) TokenID() TokenID {
	return n.token.ID()
}

// Token returns a CLONE of the underlying sda.Token struct.
// Useful for unit testing.
func (n *TreeNodeInstance) Token() *Token {
	return n.token.Clone()
}

// List returns the list of TreeNodes cached in the node (creating it if necessary)
func (n *TreeNodeInstance) List() []*TreeNode {
	n.mtx.Lock()
	if n.treeNodeList == nil {
		n.treeNodeList = n.Tree().List()
	}
	n.mtx.Unlock()
	return n.treeNodeList
}

// Index returns the index of the node in the Roster
func (n *TreeNodeInstance) Index() int {
	return n.TreeNode().RosterIndex
}

// Broadcast sends a given message from the calling node directly to all other TreeNodes
func (n *TreeNodeInstance) Broadcast(msg interface{}) error {
	for _, node := range n.List() {
		if node != n.TreeNode() {
			if err := n.SendTo(node, msg); err != nil {
				return err
			}
		}
	}
	return nil
}

// Multicast ... XXX: should probably have a parallel more robust version like "SendToChildrenInParallel"
func (n *TreeNodeInstance) Multicast(msg interface{}, nodes ...*TreeNode) error {
	for _, node := range nodes {
		if err := n.SendTo(node, msg); err != nil {
			return err
		}
	}
	return nil
}

// SendToParent sends a given message to the parent of the calling node (unless it is the root)
func (n *TreeNodeInstance) SendToParent(msg interface{}) error {
	if n.IsRoot() {
		return nil
	}
	log.Lvl4(n.Name(), strings.Split(log.Stack(), "\n")[7], "Sends to",
		n.Parent().Name())
	return n.SendTo(n.Parent(), msg)
}

// SendToChildren sends a given message to all children of the calling node.
// It stops sending if sending to one of the children fails. In that case it
// returns an error. If the underlying node is a leaf node this function does
// nothing.
func (n *TreeNodeInstance) SendToChildren(msg interface{}) error {
	if n.IsLeaf() {
		return nil
	}
	for _, node := range n.Children() {
		if err := n.SendTo(node, msg); err != nil {
			return err
		}
	}
	return nil
}

// SendToChildrenInParallel sends a given message to all children of the calling
// node. It has the following differences to node.SendToChildren:
// The actual sending happens in a go routine (in parallel).
// It continues sending to the other nodes if sending to one of the children
// fails. In that case it will collect all errors (separated by '\n'.)
// If the underlying node is a leaf node this function does
// nothing.
func (n *TreeNodeInstance) SendToChildrenInParallel(msg interface{}) error {
	if n.IsLeaf() {
		return nil
	}
	children := n.Children()
	errs := make([]collectedErrors, 0, len(children))
	eMut := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, node := range children {
		name := node.Name()
		wg.Add(1)
		go func(n2 *TreeNode) {
			if err := n.SendTo(n2, msg); err != nil {
				eMut.Lock()
				errs = append(errs, collectedErrors{name, err})
				eMut.Unlock()
			}
			wg.Done()
		}(node)
	}
	wg.Wait()
	return collectErrors("Error while sending to %s: %s\n", errs)
}

// CreateProtocol makes SDA instantiates a new protocol of name "name" and
// returns it with any error that might have happened during the creation. This
// protocol is only handled by SDA, no service are "attached" to it.
func (n *TreeNodeInstance) CreateProtocol(name string, t *Tree) (ProtocolInstance, error) {
	pi, err := n.overlay.CreateProtocolSDA(name, t)
	return pi, err
}

// Host returns the underlying Host of this node.
// WARNING: you should not play with that feature unless you know what you are
// doing. This feature is mean to access the low level parts of the API. For
// example it is used to add a new tree config / new entity list to the Conode.
func (n *TreeNodeInstance) Host() *Conode {
	return n.overlay.conode
}

// TreeNodeInstance returns itself (XXX quick hack for this services2 branch
// version for the tests)
func (n *TreeNodeInstance) TreeNodeInstance() *TreeNodeInstance {
	return n
}
func (n *TreeNodeInstance) isBound() bool {
	return n.instance != nil
}

func (n *TreeNodeInstance) bind(pi ProtocolInstance) {
	n.instance = pi
}
