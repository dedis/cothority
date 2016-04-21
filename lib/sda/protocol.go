package sda

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
)

// ProtocolID uniquely identifies a protocol
type ProtocolID uuid.UUID

// protocols holds a map of all available protocols and how to create an
// instance of it
var protocols map[ProtocolID]NewProtocol

// ProtocolInstance is the interface that instances have to use in order to be
// recognized as protocols
type ProtocolInstance interface {
	// Start is called when a leader has created its tree configuration and
	// wants to start a protocol, it calls host.StartProtocol(protocolID), that
	// in turns instantiate a new protocol (with a fresh token), and then call
	// Start on it.
	Start() error
	// Dispatch is called at the beginning by SDA for listening on the channels
	// XXX Should remove that => not using ncessarily channels
	Dispatch() error

	// DispatchMsg is a method that is called each time a message arrive for
	// this protocolInstance. TreeNodeInstance implements that method for you
	// using channels or handlers.
	DispatchMsg(*Data)
	// ProtocolInstance must be using a TreeNodeInstance so SDA knows how to
	// route the message from / to this PI.
	//TreeNodeInstance() *TreeNodeInstance
	// XXX TEMP SOLUTION
	Token() *Token
	// Shutdown cleans up the resources used by this protocol instance
	Shutdown() error
}

// NewProtocol is the function-signature needed to instantiate a new protocol
type NewProtocol func(*TreeNodeInstance) (ProtocolInstance, error)

// ProtocolRegister takes a protocol and registers it under a given uuid.
// As this might be called from an 'init'-function, we need to check the
// initialisation of protocols here and not in our own 'init'.
func ProtocolRegister(protoID ProtocolID, protocol NewProtocol) {
	if protocols == nil {
		protocols = make(map[ProtocolID]NewProtocol)
	}
	protocols[protoID] = protocol
}

// ProtocolNameToID returns the ProtocolID corresponding to the given name
func ProtocolNameToID(name string) ProtocolID {
	url := network.NamespaceURL + "protocolname/" + name
	return ProtocolID(uuid.NewV3(uuid.NamespaceURL, url))
}

// ProtocolRegisterName is a convenience function to automatically generate
// a UUID out of the name.
func ProtocolRegisterName(name string, protocol NewProtocol) ProtocolID {
	u := ProtocolNameToID(name)
	ProtocolRegister(u, protocol)
	dbg.Lvl4("Registered", name, "to", u)
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
