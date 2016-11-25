package sda

import (
	"fmt"
	"sync"

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

// ProtocolIO is an interface that allows one protocol to completely define its
// wire protocol format while still using the Overlay. Implementations must
// provide methods to read a packet coming from the network and also to write a
// packet going to the network. A default one is provided with
// defaultProtocolIO so the regular wire-format protocol can still be used.
type ProtocolIO interface {
	// Wrap takes a message and the overlay information and returns the message
	// has to be sent directly to the network alongside with any error that
	// happened.
	// the bigger outer struct. Msg can be nil, that case it means the message
	// is only an internal message of the Overlay.
	Wrap(msg interface{}, info *OverlayMessage) (interface{}, error)
	// Unwrap takes the message coming from the network and must returns the
	// inner message that is going to be dispatched to the ProtocolInstance, the
	// OverlayMessage needed by the Overlay to function correctly and then any
	// error that might have occured.
	Unwrap(msg interface{}) (interface{}, *OverlayMessage, error)
	// PacketType returns the packet type ID that this Protocol expects from the
	// network. This is needed in order for the Overlay to receive those
	// messages and dispatch them to the correct ProtocolIO.
	PacketType() network.PacketTypeID
}

// NewProtocolIO is a function typedef to instantiate a new ProtocolIO
type NewProtocolIO func() ProtocolIO

type protocolIOFactory_ struct {
	factories map[string]NewProtocolIO
}

var protocolIOFactory = protocolIOFactory_{
	factories: make(map[string]NewProtocolIO),
}

// RegisterProtocolIO takes a name and and NewProtocolIO and save both fields.
// When a Conode is instantiated, all ProtocolIO will be generated and stored
// for this Conode.
func RegisterProtocolIO(name string, n NewProtocolIO) {
	_, present := protocolIOFactory.factories[name]
	if present {
		log.Error("protocolIOStore already registered a ProtocolIO at this name", name)
		return
	}
	protocolIOFactory.factories[name] = n
}

// protocolIOStore contains all created ProtocolIO and is generally used by the
// Overlay. It contains the default ProtocolIO used by the Overlay in order to
// still function properly in case the old wire-format protocol is used.
type protocolIOStore struct {
	sync.Mutex
	protos []ProtocolIO
	names  map[string]int
	types  map[network.PacketTypeID]int
	// the one that gets used in case no ProtocolIO is defined
	defaultIO ProtocolIO
}

func (p *protocolIOStore) getByName(name string) ProtocolIO {
	p.Lock()
	defer p.Unlock()
	idx, ok := p.names[name]
	if !ok || idx >= len(p.protos) || p.protos[idx] == nil {
		return p.defaultIO
	}
	return p.protos[idx]
}

func (p *protocolIOStore) getByPacketType(t network.PacketTypeID) ProtocolIO {
	p.Lock()
	defer p.Unlock()
	idx, ok := p.types[t]
	if !ok || idx >= len(p.protos) || p.protos[idx] == nil {
		return p.defaultIO
	}
	return p.protos[idx]
}

func newProtocolIOStore(disp network.Dispatcher, proc network.Processor) *protocolIOStore {
	pstore := &protocolIOStore{
		names:     make(map[string]int),
		types:     make(map[network.PacketTypeID]int),
		defaultIO: new(defaultProtoIO),
	}
	for name, newIO := range protocolIOFactory.factories {
		io := newIO()
		pstore.protos = append(pstore.protos, io)
		pstore.names[name] = len(pstore.protos) - 1
		pstore.types[io.PacketType()] = len(pstore.protos) - 1
		disp.RegisterProcessor(proc, io.PacketType())
		log.Lvl2("Instantiating ProtocolIO", name, "at position", len(pstore.protos))
	}
	// also add the default one
	return pstore
}
