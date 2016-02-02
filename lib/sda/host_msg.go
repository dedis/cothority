package sda

import (
	"errors"
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
	"time"
)

var timeOut = 30 * time.Second

// Our message-types used in sda
var SDADataMessage = network.RegisterMessageType(SDAData{})
var RequestTreeMessage = network.RegisterMessageType(RequestTree{})
var RequestEntityListMessage = network.RegisterMessageType(RequestEntityList{})
var SendTreeMessage = TreeMarshalType
var SendEntityListMessage = EntityListType

// SDAData is to be embedded in every message that is made for a
// ProtocolInstance
type SDAData struct {
	// Token uniquely identify the protocol instance this msg is made for
	From *Token
	// The TreeNodeId Where the message goes to
	To *Token
	// NOTE: this is taken from network.NetworkMessage
	Entity *network.Entity
	// MsgType of the underlying data
	MsgType uuid.UUID
	// The interface to the actual Data
	Msg network.ProtocolMessage
	// The actual data as binary blob
	MsgSlice []byte
}

// A Token contains all identifiers needed to Uniquely identify one protocol
// instance. It gets passed when a new protocol instance is created and get used
// by every protocol instance when they want to send a message. That way, the
// host knows how to create the SDAData message around the protocol's message
// with the right fields set.
type Token struct {
	EntityListID uuid.UUID
	TreeID       uuid.UUID
	ProtocolID   uuid.UUID
	RoundID      uuid.UUID
	TreeNodeID   uuid.UUID
	cacheId      uuid.UUID
}

// Returns the Id of a token so we can put that in a map easily
func (t *Token) Id() uuid.UUID {
	if t.cacheId == uuid.Nil {
		url := network.UuidURL + "token/" + t.EntityListID.String() +
			t.RoundID.String() + t.ProtocolID.String() + t.TreeID.String() +
			t.TreeNodeID.String()
		t.cacheId = uuid.NewV5(uuid.NamespaceURL, url)
	}
	return t.cacheId
}

// Return a new Token contianing a reference to the given TreeNode
func (t *Token) ChangeTreeNodeID(newid uuid.UUID) *Token {
	t_other := *t
	t_other.TreeNodeID = newid
	t_other.cacheId = uuid.Nil
	return &t_other
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
