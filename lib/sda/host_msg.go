package sda

import (
	"errors"
	"github.com/dedis/cothority/lib/network"
	. "github.com/satori/go.uuid"
)

// init registers all our message-types to the network-interface
func init() {
	network.RegisterProtocolType(SDAMessageType, SDAMessage{})
	network.RegisterProtocolType(RequestTreeType, RequestTree{})
	network.RegisterProtocolType(SendTreeType, TreeNode{})
	network.RegisterProtocolType(RequestIdentityListType, RequestIdentityList{})
	network.RegisterProtocolType(SendIdentityListType, IdentityList{})
	network.RegisterProtocolType(IdentityMessageType, IdentityMessage{})
}

// constants used for the message-types
const (
	SDAMessageType = iota + 10
	RequestTreeType
	SendTreeType
	RequestIdentityListType
	SendIdentityListType
	IdentityMessageType
)

// ProtocolInfo is to be embedded in every message that is made for a
// ProtocolInstance
type SDAMessage struct {
	// The ID of the protocol
	ProtoID UUID
	// The ID of the protocol instance - the counter
	InstanceID UUID

	// MsgType of the underlying data
	MsgType network.Type
	// The interface to the actual Data
	Data network.ProtocolMessage
	// The actual data as binary blob
	DataSlice []byte
}

// RequestTree is used to ask the parent for a given Tree
type RequestTree struct {
	// The treeID of the tree we want
	TreeID UUID
}

// RequestIdentityList is used to ask the parent for a given IdentityList
type RequestIdentityList struct {
	IdentityListID UUID
}

// IdentityMessage is the first message we send on creation of a link
type IdentityMessage struct {
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
