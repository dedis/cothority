package sda

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/satori/go.uuid"
)

// ProtocolInstance is the interface that instances have to use in order to be
// recognized as protocols
type ProtocolInstance interface {
	// Start is called when a leader has created its tree configuration and
	// wants to start a protocol, it calls host.StartProtocol(protocolID), that
	// in turns instantiate a new protocol (with a fresh token) , and then call
	// Start on it.
	Start()
	// Dispatch is called whenever packets are ready and should be treated
	Dispatch(m []*SDAData) error
}

// NewProtocol is the function-signature needed to instantiate a new protocol
type NewProtocol func(*Host, *Tree, *Token) ProtocolInstance

// protocols holds a map of all available protocols and how to create an
// instance of it
var protocols map[uuid.UUID]NewProtocol

// ProtocolMapper handles the mapping between tokens and protocol instances. It
// also provides helpers for protocol instances such as sending a message to
// someone only requires to give the token and the message and protocolmapper
// will handle the rest.
// NOTE: This protocolMapper handle only a few things now but I suggest we leave
// it there when our Host struct will grow. As it is starting to be already big,
// we may, in the future, move many protocol instance /  tree / entitylist
// handling methods in here, so host won't be too much .."overloaded" like our
// old sign.Node. Host could relay everything that is realted to that in this
// struct and handles the reste such as the connection, the callbacks, the
// errors handling etc.
type protocolMapper struct {
	// mapping instances with their tokens
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
// it returns true if it has successfullyy dispatched the msg or false
// otherwise. It can return false because it want to aggregate some messages
// until every children of this host has sent their messages.
func (pm *protocolMapper) DispatchToInstance(sda *SDAData) bool {
	var pi ProtocolInstance
	if pi = pm.Instance(&sda.Token); pi == nil {
		dbg.Lvl2("No instance for this token")
		return false
	}
	//  Get the node corresponding to this host in the Tree
	node := pm.Host.TreeNode(sda.Token.TreeID)
	if node == nil {
		dbg.Error("Could not find TreeNode for this host in aggregate")
		return false
	}
	// if message comes from parent, dispatch directly
	if !node.IsRoot() && sda.Entity.Equal(node.Parent.Entity) {
		pi.Dispatch([]*SDAData{sda})
		return true
	}

	// if messages come from children we must aggregate them
	var msgs []*SDAData
	var ok bool
	// if we still need to wait additionals message, we return
	if msgs, ok = pm.aggregate(node, sda); !ok {
		dbg.Lvl2("Still aggregating for this SDAData")
		return false
	}
	// all is good
	pi.Dispatch(msgs)
	return true
}

// aggregate store the message for a protocol instance such that a protocol
// instances will get all its children messages at once.
// node is the node the host is representing in this Tree, and sda is the
// message being analyzed.
func (pm *protocolMapper) aggregate(node *TreeNode, sda *SDAData) ([]*SDAData, bool) {
	// store the msg
	tokId := sda.Token.Id()
	if _, ok := pm.msgQueue[tokId]; !ok {
		pm.msgQueue[tokId] = make([]*SDAData, 0)
	}
	msgs := append(pm.msgQueue[tokId], sda)
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
	dbg.Lvl2("Len(msg)=", len(msgs), " vs len(children)=", len(node.Children))
	return nil, false
}

// ProtocolRegister takes a protocol and registers it under a given name.
// As this might be called from an 'init'-function, we need to check the
// initialisation of protocols here and not in our own 'init'.
func ProtocolRegister(protoID uuid.UUID, protocol NewProtocol) {
	if protocols == nil {
		protocols = make(map[uuid.UUID]NewProtocol)
	}
	protocols[protoID] = protocol
}

// ProtocolExists returns whether a certain protocol already has been
// registered
func ProtocolExists(protoID uuid.UUID) bool {
	_, ok := protocols[protoID]
	return ok
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
	pm.instances[tok.Id()] = proto
}
