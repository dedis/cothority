package sda

import (
	"bytes"
	"encoding/json"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
)

// Our message-types used in sda
var SDADataMessage = network.RegisterMessageType(SDAData{})
var RequestTreeMessage = network.RegisterMessageType(RequestTree{})
var RequestEntityListMessage = network.RegisterMessageType(RequestEntityList{})
var SendTreeMessage = TreeMarshalType
var SendEntityListMessage = EntityListType

var CosiRequestMessage = network.RegisterMessageType(CosiRequest{})
var CosiResponseMessage = network.RegisterMessageType(CosiResponse{})

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

// Return a new Token containing a reference to the given TreeNode
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

// CLI Part of SDA

// The Request to CoSi: it is used by the client to send something to sda that
// will in turn give that to the CoSi system. It contains the message the client
// wants to sign.
type CosiRequest struct {
	// The entity list to use for creating the cosi tree
	EntityList *EntityList
	// the actual message to sign by CoSi.
	Message []byte
}

// CoSiResponse contains the signature out of the CoSi system.
// It can be verified using the lib/cosi package.
// NOTE: the `suite` field is absent here because this struct is a temporary
// hack and we only supports one suite for the moment,i.e. ed25519.
type CosiResponse struct {
	// The Challenge out a of the Multi Schnorr signature
	Challenge abstract.Secret `json:",string"`
	// the Response out of the Multi Schnorr Signature
	Response abstract.Secret `json:",string"`
}

func (s *CosiResponse) MarshalJSON() ([]byte, error) {
	cw := new(bytes.Buffer)
	rw := new(bytes.Buffer)
	cliutils.WriteSecret64(network.Suite, cw, s.Challenge)
	cliutils.WriteSecret64(network.Suite, rw, s.Response)
	return json.Marshal(struct {
		Challenge string
		Response  string
	}{Challenge: cw.String(),
		Response: rw.String()})
}
