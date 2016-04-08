package sda

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"

	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
)

// Our message-types used in sda

var CosiRequestMessage = network.RegisterMessageType(CosiRequest{})
var CosiResponseMessage = network.RegisterMessageType(CosiResponse{})

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

// A Token contains all identifiers needed to uniquely identify one protocol
// instance. It gets passed when a new protocol instance is created and get used
// by every protocol instance when they want to send a message. That way, the
// host knows how to create the SDAData message around the protocol's message
// with the right fields set.
type Token struct {
	EntityListID EntityListID
	TreeID       TreeID
	ProtoID      ProtocolID
	RoundID      RoundID
	TreeNodeID   TreeNodeID
	cacheId      TokenID
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
			t.RoundID.String() + t.ProtoID.String() + t.TreeID.String() +
			t.TreeNodeID.String()
		t.cacheId = TokenID(uuid.NewV5(uuid.NamespaceURL, url))
	}
	return t.cacheId
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

// CLI Part of SDA

// CosiRequest is used by the client to send something to sda that
// will in turn give that to the CoSi system.
// It contains the message the client wants to sign.
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
	// The hash of the signed statement
	Sum []byte
	// The Challenge out a of the Multi Schnorr signature
	Challenge abstract.Secret
	// the Response out of the Multi Schnorr Signature
	Response abstract.Secret
}

// MarshalJSON implements golang's JSON marshal interface
// XXX might be moved to another package soon
func (s *CosiResponse) MarshalJSON() ([]byte, error) {
	cw := new(bytes.Buffer)
	rw := new(bytes.Buffer)

	err := crypto.WriteSecret64(network.Suite, cw, s.Challenge)
	if err != nil {
		return nil, err
	}
	err = crypto.WriteSecret64(network.Suite, rw, s.Response)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Sum       string
		Challenge string
		Response  string
	}{
		Sum:       base64.StdEncoding.EncodeToString(s.Sum),
		Challenge: cw.String(),
		Response:  rw.String(),
	})
}

// UnmarshalJSON implements golang's JSON unmarshal interface
func (s *CosiResponse) UnmarshalJSON(data []byte) error {
	type Aux struct {
		Sum       string
		Challenge string
		Response  string
	}
	aux := &Aux{}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	var err error
	if s.Sum, err = base64.StdEncoding.DecodeString(aux.Sum); err != nil {
		return err
	}
	suite := network.Suite
	cr := strings.NewReader(aux.Challenge)
	if s.Challenge, err = crypto.ReadSecret64(suite, cr); err != nil {
		return err
	}
	rr := strings.NewReader(aux.Response)
	if s.Response, err = crypto.ReadSecret64(suite, rr); err != nil {
		return err
	}
	return nil
}
