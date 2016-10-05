package template

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*sda.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

import "github.com/dedis/cothority/sda"

// Name can be used from other packages to refer to this protocol.
const Name = "Template"

// Announce is used to pass a message to all children.
type Announce struct {
	Message string
}

// StructAnnounce just contains Announce and the data necessary to identify and
// process the message in the sda framework.
type StructAnnounce struct {
	*sda.TreeNode
	Announce
}

// Reply returns the count of all children.
type Reply struct {
	ChildrenCount int
}

// StructReply just contains Reply and the data necessary to identify and
// process the message in the sda framework.
type StructReply struct {
	*sda.TreeNode
	Reply
}
