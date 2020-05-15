package byzcoin

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

import "go.dedis.ch/onet/v3"

// Name can be used from other packages to refer to this protocol.
const Name = "Rollup"

// Announce is used to pass a message to all children.
type Announce struct {
	Message string
}

// announceWrapper just contains Announce and the data necessary to identify
// and process the message in onet.
type announceWrapper struct {
	*onet.TreeNode
	Announce
}

// Reply returns the count of all children.
type Reply struct {
	ChildrenCount int
	Message       string
}

// replyWrapper just contains Reply and the data necessary to identify and
// process the message in onet.
type replyWrapper struct {
	*onet.TreeNode
	Reply
}
