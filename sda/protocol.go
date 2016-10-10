package sda

import (
	"fmt"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/satori/go.uuid"
)

// ProtocolID uniquely identifies a protocol
type ProtocolID uuid.UUID

// String returns canonical string representation of the ID
func (pid ProtocolID) String() string {
	return uuid.UUID(pid).String()
}

// NewProtocol is the function-signature needed to instantiate a new protocol
type NewProtocol func(*TreeNodeInstance) (ProtocolInstance, error)

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

var protocols = newProtocolStorage()

// protocolStorage holds all protocols either globally or per-Conode.
type protocolStorage struct {
	// Instantiators maps the name of the protocols to the `NewProtocol`-
	// methods.
	instantiators map[string]NewProtocol
}

// newProtocolStorage returns an initialized ProtocolStorage-struct.
func newProtocolStorage() *protocolStorage {
	return &protocolStorage{
		instantiators: map[string]NewProtocol{},
	}
}

// ProtocolIDToName returns the name to the corresponding protocolID.
func (ps *protocolStorage) ProtocolIDToName(id ProtocolID) string {
	for n := range ps.instantiators {
		if id == ProtocolNameToID(n) {
			return n
		}
	}
	return ""
}

// ProtocolExists returns whether a certain protocol already has been
// registered.
func (ps *protocolStorage) ProtocolExists(protoID ProtocolID) bool {
	_, ok := ps.instantiators[ps.ProtocolIDToName(protoID)]
	return ok
}

// Register takes a name and a NewProtocol and stores it in the structure.
// If the protocol already exists, a warning is printed and the NewProtocol is
// *not* stored.
func (ps *protocolStorage) Register(name string, protocol NewProtocol) (ProtocolID, error) {
	id := ProtocolNameToID(name)
	if _, exists := ps.instantiators[name]; exists {
		return ProtocolID(uuid.Nil),
			fmt.Errorf("Protocol -%s- already exists - not overwriting", name)
	}
	ps.instantiators[name] = protocol
	log.Lvl4("Registered", name, "to", id)
	return id, nil
}

// ProtocolNameToID returns the ProtocolID corresponding to the given name.
func ProtocolNameToID(name string) ProtocolID {
	url := network.NamespaceURL + "protocolname/" + name
	return ProtocolID(uuid.NewV3(uuid.NamespaceURL, url))
}

// GlobalProtocolRegister registers a protocol in the global namespace.
// This is used in protocols that register themselves in the `init`-method.
// All registered protocols will be copied to every instantiated Conode. If a
// protocol is tied to a service, use `Conode.ProtocolRegisterName`
func GlobalProtocolRegister(name string, protocol NewProtocol) (ProtocolID, error) {
	return protocols.Register(name, protocol)
}
