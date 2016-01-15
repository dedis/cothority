package sda

import (
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
)

// protocols holds a map of all available protocols and how to create an
// instance of it
var protocols map[uuid.UUID]NewProtocol

// ProtocolInstance is the interface that instances have to use in order to be
// recognized as protocols
type ProtocolInstance interface {
	// Start is called when a leader has created its tree configuration and
	// wants to start a protocol, it calls host.StartProtocol(protocolID), that
	// in turns instantiate a new protocol (with a fresh token) , and then call
	// Start on it.
	Start() error
	// Dispatch is called whenever packets are ready and should be treated
	Dispatch([]*SDAData) error
}

// NewProtocol is the function-signature needed to instantiate a new protocol
type NewProtocol func(*Host, *TreeNode, *Token) ProtocolInstance

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
	// maps token-uid|treenode-uid to ProtocolInstances
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
	var msgs []*SDAData
	var ok bool
	// if we still need to wait additionals message, we return
	if msgs, ok = pm.aggregate(node, sdaMsg); !ok {
		return false, errors.New("Still aggregating for this SDAData")
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
	dbg.Lvl2("Len(msg)=", len(msgs), " vs len(children)=", len(node.Children))
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

// ProtocolStruct combines a host, treeNode and a send-function as convenience
type ProtocolStruct struct {
	*Host
	*TreeNode
	Token *Token
}

// NewProtocolStruct creates a new structure
func NewProtocolStruct(h *Host, t *TreeNode, tok *Token) *ProtocolStruct {
	return &ProtocolStruct{h, t, tok}
}

// Send takes the message and sends it to the given TreeNode
func (ps *ProtocolStruct) Send(to *TreeNode, msg network.ProtocolMessage) error {
	return ps.Host.SendSDAToTreeNode(ps.Token, to, msg)
}

// ProtocolRegister takes a protocol and registers it under a given uuid.
// As this might be called from an 'init'-function, we need to check the
// initialisation of protocols here and not in our own 'init'.
func ProtocolRegister(protoID uuid.UUID, protocol NewProtocol) {
	if protocols == nil {
		protocols = make(map[uuid.UUID]NewProtocol)
	}
	protocols[protoID] = protocol
}

func ProtocolNameToUuid(name string) uuid.UUID {
	url := "http://dedis.epfl.ch/protocolname/" + name
	return uuid.NewV3(uuid.NamespaceURL, url)
}

// ProtocolRegisterName is a convenience function to automatically generate
// a UUID out of the name.
func ProtocolRegisterName(name string, protocol NewProtocol) {
	ProtocolRegister(ProtocolNameToUuid(name), protocol)
}

// ProtocolExists returns whether a certain protocol already has been
// registered
func ProtocolExists(protoID uuid.UUID) bool {
	_, ok := protocols[protoID]
	return ok
}
