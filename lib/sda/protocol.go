package sda

import (
	"errors"
	"github.com/satori/go.uuid"
)

// NewProtocol is the function-signature needed to instantiate a new protocol
type NewProtocol func(*Host, *Tree) ProtocolInstance

// protocols holds a map of all available protocols and how to create an
// instance of it
var protocols map[uuid.UUID]NewProtocol

// Protocol is the interface that instances have to use in order to be
// recognized as protocols
type ProtocolInstance interface {
	// Dispatch is called whenever packets are ready and should be treated
	Dispatch(m *SDAData) error
	Id() uuid.UUID
}

// ProtocolInstantiate creates a new instance of a protocol given by it's name
func ProtocolInstantiate(protoID uuid.UUID, n *Host, t *Tree) (ProtocolInstance, error) {
	p, ok := protocols[protoID]
	if !ok {
		return nil, errors.New("Protocol doesn't exist")
	}
	return p(n, t), nil
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
