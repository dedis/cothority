package sda

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
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
	// in turns instantiate a new protocol (with a fresh token), and then call
	// Start on it.
	Start() error
	// Dispatch is called as a go-routine and can be used to handle channels
	Dispatch() error
	// Shutdown cleans up the resources used by this protocol instance
	Shutdown() error

	// HACKY / TEMPORARY way: need to access protocols/cosi from sda (without
	// cycle import). It will be solved in the next release.
	// give the message to sign to the protocol instance
	SigningMessage(msg []byte)
	// callback to register when the signature is done
	RegisterDoneCallback(func(chal, secret abstract.Secret))
}

// NewProtocol is the function-signature needed to instantiate a new protocol
type NewProtocol func(*Node) (ProtocolInstance, error)

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
	url := network.UuidURL + "protocolname/" + name
	return uuid.NewV3(uuid.NamespaceURL, url)
}

// ProtocolRegisterName is a convenience function to automatically generate
// a UUID out of the name.
func ProtocolRegisterName(name string, protocol NewProtocol) uuid.UUID {
	u := ProtocolNameToUuid(name)
	ProtocolRegister(u, protocol)
	dbg.Lvl4("Registered", name, "to", u)
	return u
}

// ProtocolExists returns whether a certain protocol already has been
// registered
func ProtocolExists(protoID uuid.UUID) bool {
	_, ok := protocols[protoID]
	return ok
}
