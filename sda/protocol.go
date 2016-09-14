package sda

import (
	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/satori/go.uuid"
)

// ProtocolID uniquely identifies a protocol
type ProtocolID uuid.UUID

// protocols holds a map of all available protocols and how to create an
// instance of it
var protocols map[ProtocolID]NewProtocol
var protocolNames map[ProtocolID]string

// ProtocolInstance is the interface that instances have to use in order to be
// recognized as protocols
type ProtocolInstance interface {
	// Start is called when a leader has created its tree configuration and
	// wants to start a protocol, it calls host.StartProtocol(protocolID), that
	// in turns instantiate a new protocol (with a fresh token), and then call
	// Start on it.
	Start() error
	// Dispatch is called at the beginning by SDA for listening on the channels
	Dispatch() error

	// DispatchMsg is a method that is called each time a message arrives for
	// this protocolInstance. TreeNodeInstance implements that method for you
	// using channels or handlers.
	ProcessProtocolMsg(*ProtocolMsg)
	// The token representing this ProtocolInstance
	Token() *Token
	// Shutdown cleans up the resources used by this protocol instance
	Shutdown() error
}

// NewProtocol is the function-signature needed to instantiate a new protocol
type NewProtocol func(*TreeNodeInstance) (ProtocolInstance, error)

// ProtocolNameToID returns the ProtocolID corresponding to the given name
func ProtocolNameToID(name string) ProtocolID {
	url := network.NamespaceURL + "protocolname/" + name
	return ProtocolID(uuid.NewV3(uuid.NamespaceURL, url))
}

// ProtocolIDToName returns the name to the corresponding protocolID
func ProtocolIDToName(id ProtocolID) string {
	return protocolNames[id]
}

// ProtocolRegisterName is a convenience function to automatically generate
// a UUID out of the name. If the protocol is alreday registere, it will
// output an error.
// Take care if you want to register the same protocol-name for different
// service-instantiations, as you might run into problems when doing
// tests and/or simulations.
func ProtocolRegisterName(name string, protocol NewProtocol) ProtocolID {
	u := ProtocolNameToID(name)
	if protocols == nil {
		protocols = make(map[ProtocolID]NewProtocol)
		protocolNames = make(map[ProtocolID]string)
	}
	if n, exists := protocolNames[u]; exists {
		log.Warn("Protocol", n, "already exists - not overwriting")
		return u
	}
	protocolNames[u] = name
	protocols[u] = protocol
	log.LLvl4("Registered", name, "to", u)
	return u
}

// ProtocolExists returns whether a certain protocol already has been
// registered
func ProtocolExists(protoID ProtocolID) bool {
	_, ok := protocols[protoID]
	return ok
}

// String returns canonical string representation of the ID
func (pid ProtocolID) String() string {
	return uuid.UUID(pid).String()
}

// ProtocolInstantiate instantiate a protocol from its ID
func ProtocolInstantiate(protoID ProtocolID, tni *TreeNodeInstance) (ProtocolInstance, error) {
	fn, ok := protocols[protoID]
	if !ok {
		return nil, errors.New("No protocol constructor with this ID")
	}
	return fn(tni)
}
