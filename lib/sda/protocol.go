package sda

import (
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
	Start()
	// Dispatch is called whenever packets are ready and should be treated
	Dispatch(m *SDAData) error
}

// NewProtocol is the function-signature needed to instantiate a new protocol
type NewProtocol func(*Host, *TreeNode, *Token) ProtocolInstance

// ProtocolMapper handles the mapping between tokens and protocol instances. It
// also provides helpers for protocol instances such as sending a message to
// someone only requires to give the token and the message and protocolmapper
// will handle the rest.
type protocolMapper struct {
	// mapping instances with their tokens
	// maps token-uid|treenode-uid to ProtocolInstances
	instances map[uuid.UUID]ProtocolInstance
	// aggregate messages in order to dispatch them at once in the protocol
	// instance
	msgQueue map[uuid.UUID][]*SDAData
}

func newProtocolMapper() *protocolMapper {
	return &protocolMapper{
		instances: make(map[uuid.UUID]ProtocolInstance),
	}
}

func (pm *protocolMapper) DispatchToInstance(sda *SDAData) bool {
	var pi ProtocolInstance
	if pi = pm.Instance(&sda.Token); pi == nil {
		return false
	}
	// TODO aggregate msg if necessary
	//
	// Dispatch msg
	pi.Dispatch(sda)
	return true
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
func (pm *protocolMapper) RegisterProtocolInstance(proto ProtocolInstance, tok *Token, tn *TreeNode) {
	// And registers it
	pm.instances[uuid.And(tok.Id(), tn.Id)] = proto
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

// Send adds the token
func (ps *ProtocolStruct) Send(to *network.Entity, msg network.NetworkMessage) {
	ps.Host.Send(ps.Token, to, msg)
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

// ProtocolRegisterName is a convenience function to automatically generate
// a UUID out of the name.
func ProtocolRegisterName(name string, protocol NewProtocol) {
	url := "http://dedis.epfl.ch/protocolname/" + name
	uuid := uuid.NewV3(uuid.NamespaceURL, url)
	ProtocolRegister(uuid, protocol)
}

// ProtocolExists returns whether a certain protocol already has been
// registered
func ProtocolExists(protoID uuid.UUID) bool {
	_, ok := protocols[protoID]
	return ok
}
