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
	Start() error
	// Dispatch is called whenever packets are ready and should be treated
	Dispatch([]*SDAData) error
}

// NewProtocol is the function-signature needed to instantiate a new protocol
type NewProtocol func(*Host, *TreeNode, *Token) ProtocolInstance

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
