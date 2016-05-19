package sda

import (
	"sync"

	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
)

// SDAData is to be embedded in every message that is made for a
// ID of SDAData message as registered in network
var SDADataMessageID = network.RegisterMessageType(Data{})

// ID of RequestTree message as registered in network
var RequestTreeMessageID = network.RegisterMessageType(RequestTree{})

// ID of RequestEntityList message as registered in network
var RequestEntityListMessageID = network.RegisterMessageType(RequestEntityList{})

// ID of TreeMarshal message as registered in network
var SendTreeMessageID = TreeMarshalTypeID

// ID of EntityList message as registered in network
var SendEntityListMessageID = EntityListTypeID

// Data is to be embedded in every message that is made for a
// ProtocolInstance
type Data struct {
	// Token uniquely identify the protocol instance this msg is made for
	From *Token
	// The TreeNodeId Where the message goes to
	To *Token
	// NOTE: this is taken from network.NetworkMessage
	Entity *network.Entity
	// MsgType of the underlying data
	MsgType network.MessageTypeID
	// The interface to the actual Data
	Msg network.ProtocolMessage
	// The actual data as binary blob
	MsgSlice []byte
	// Config the actual config
	Config GenericConfig
}

// RoundID uniquely identifies a round of a protocol run
type RoundID uuid.UUID

// String returns the canonical representation of the rounds ID (wrapper around
// uuid.UUID.String())
func (rId RoundID) String() string {
	return uuid.UUID(rId).String()
}

// TokenID uniquely identifies the start and end-point of a message by an ID
// (see Token struct)
type TokenID uuid.UUID

func (t *TokenID) String() string {
	return uuid.UUID(*t).String()
}

// A Token contains all identifiers needed to uniquely identify one protocol
// instance. It gets passed when a new protocol instance is created and get used
// by every protocol instance when they want to send a message. That way, the
// host knows how to create the SDAData message around the protocol's message
// with the right fields set.
type Token struct {
	EntityListID EntityListID
	TreeID       TreeID
	// TO BE REMOVED
	ProtoID   ProtocolID
	ServiceID ServiceID
	RoundID   RoundID
	// TreeNodeID is defined by the
	TreeNodeID TreeNodeID
	cacheId    TokenID
}

// Global mutex when we're working on Tokens. Needed because we
// copy Tokens in ChangeTreeNodeID.
var tokenMutex sync.Mutex

// Id returns the TokenID which can be used to identify by token in map
func (t *Token) Id() TokenID {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	if t.cacheId == TokenID(uuid.Nil) {
		url := network.NamespaceURL + "token/" + t.EntityListID.String() +
			t.RoundID.String() + t.ServiceID.String() + t.ProtoID.String() + t.TreeID.String() +
			t.TreeNodeID.String()
		t.cacheId = TokenID(uuid.NewV5(uuid.NamespaceURL, url))
	}
	return t.cacheId
}

// Clone returns a new token out of this one
func (t *Token) Clone() *Token {
	t2 := *t
	return &t2
}

// ChangeTreeNodeID return a new Token containing a reference to the given
// TreeNode
func (t *Token) ChangeTreeNodeID(newid TreeNodeID) *Token {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	t_other := *t
	t_other.TreeNodeID = newid
	t_other.cacheId = TokenID(uuid.Nil)
	return &t_other
}

// RequestTree is used to ask the parent for a given Tree
type RequestTree struct {
	// The treeID of the tree we want
	TreeID TreeID
}

// RequestEntityList is used to ask the parent for a given EntityList
type RequestEntityList struct {
	EntityListID EntityListID
}

// EntityListUnknown is used in case the entity list is unknown
type EntityListUnknown struct {
}

// SendEntity is the first message we send on creation of a link
type SendEntity struct {
	Name string
}
