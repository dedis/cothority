package onchain

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/share/dkg"
	"gopkg.in/dedis/onet.v1"
)

// Name can be used from other packages to refer to this protocol.
const Name = "OnchainSecret"

// Init asks all nodes to set up a private/public key pair. It is sent to
// the next node in the
type Init struct {
}

// InitReply returns the public key of that node.
type InitReply struct {
	Public abstract.Point
}

// StartDeal is used by the leader to initiate the Deals.
type StartDeal struct {
	Publics   []abstract.Point
	Threshold uint
}

// Deal sends the deals for the shared secret.
type Deal struct {
	Deal *dkg.Deal
}

//

// Announce is used to pass a message to all children.
type Announce struct {
	Message string
}

// StructAnnounce just contains Announce and the data necessary to identify and
// process the message in the sda framework.
type StructAnnounce struct {
	*onet.TreeNode
	Announce
}

// Reply returns the count of all children.
type Reply struct {
	ChildrenCount int
}

// StructReply just contains Reply and the data necessary to identify and
// process the message in the sda framework.
type StructReply struct {
	*onet.TreeNode
	Reply
}
