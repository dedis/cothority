package sda

import (
	"github.com/satori/go.uuid"
)

// NewProtocol is the function-signature needed to instantiate a new protocol
type NewProtocol func(*Host, *Tree, *Token) ProtocolInstance

// ProtocolMapper handles the mapping between tokens and protocol instances. It
// also provides helpers for protocol instances such as sending a message to
// someone only requires to give the token and the message and protocolmapper
// will handle the rest.
type protocolMapper struct {
	// mapping instances with their tokens
	instances map[uuid.UUID]ProtocolInstance
}

func newProtocolMapper() *protocolMapper {
	return &protocolMapper{
		instances: make(map[uuid.UUID]ProtocolInstance),
	}
}

// protocols holds a map of all available protocols and how to create an
// instance of it
var protocols map[uuid.UUID]NewProtocol

// Protocol is the interface that instances have to use in order to be
// recognized as protocols
type ProtocolInstance interface {
	// Start is called when a leader has created its tree configuration and
	// wants to start a protocol, it calls host.StartProtocol(protocolID), that
	// in turns instantiate a new protocol (with a fresh token) , and then call
	// Start on it.
	Start()
	// Dispatch is called whenever packets are ready and should be treated
	Dispatch(m *SDAData) error
	Id() uuid.UUID
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

// RegisterProtocolInstance simply put the proto instance mapping with the token
func (pm *protocolMapper) RegisterProtocolInstance(proto ProtocolInstance, tok *Token) {
	// first set the id of the protocol INSTANCE in the token ( we dont know
	// before creating the protocol instance itself)
	tok.InstanceID = proto.Id()
	// then registers it.
	pm.instances[tok.Id()] = proto
}
