package sda

import (
	"errors"
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
)

// init registers all our message-types to the network-interface
func init() {
	network.RegisterProtocolType(SDADataMessage, SDAData{})
	network.RegisterProtocolType(RequestTreeMessage, RequestTree{})
	network.RegisterProtocolType(RequestEntityListMessage, RequestEntityList{})
	network.RegisterProtocolType(SendEntityListMessage, EntityList{})
}

// constants used for the message-types
const (
	SDADataMessage = iota + 10
	RequestTreeMessage
	RequestEntityListMessage
	SendEntityListMessage = EntityListType
	SendTreeMessage       = TreeMarshalType
)

// SDAData is to be embedded in every message that is made for a
// ProtocolInstance
type SDAData struct {
	// Token uniquely identify the protocol instance this msg is made for
	Token
	// NOTE: this is taken from network.NetworkMessage
	Entity *network.Entity
	// MsgType of the underlying data
	MsgType network.Type
	// The interface to the actual Data
	Msg network.ProtocolMessage
	// The actual data as binary blob
	MsgSlice []byte
}

// A Token contains all identifiers needed to Uniquely identify one protocol
// instance. It get passed when a new protocol instance is created and get used
// by every protocol instance when they want to send a message. That way, the
// host knows how to create the SDAData message around the protocol's message
// with the right fields set.
// It's kinda "authentify" one protocol instance, hence the Token name.
type Token struct {
	EntityListID uuid.UUID
	TreeID       uuid.UUID
	ProtocolID   uuid.UUID
	InstanceID   uuid.UUID
}

// Returns the Id of a token so we can put that in a map easily
func (t *Token) Id() uuid.UUID {
	return uuid.And(uuid.And(t.EntityListID, t.TreeID), uuid.And(t.ProtocolID, t.InstanceID))
}

// RequestTree is used to ask the parent for a given Tree
type RequestTree struct {
	// The treeID of the tree we want
	TreeID uuid.UUID
}

// RequestEntityList is used to ask the parent for a given EntityList
type RequestEntityList struct {
	EntityListID uuid.UUID
}

// In case the entity list is unknown
type EntityListUnknown struct {
}

// SendEntity is the first message we send on creation of a link
type SendEntity struct {
	Name string
}

// IPType defines how incoming packets are handled
type IPType int

const (
	WaitForAll IPType = iota
	PassDirect
	Timeout
)

// NoSuchState indicates that the given state doesn't exist in the
// chosen ProtocolInstance
var NoSuchState error = errors.New("This state doesn't exist")
