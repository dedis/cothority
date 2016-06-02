package example_handlers

import "github.com/dedis/cothority/lib/sda"

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
