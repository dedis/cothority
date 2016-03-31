package skipchain

import "github.com/dedis/cothority/lib/sda"

// MessageAnnounce is used to pass a message to all children
type MessageAnnounce struct {
	Message string
}

type StructAnnounce struct {
	*sda.TreeNode
	MessageAnnounce
}

// MessageReply returns the count of all children
type MessageReply struct {
	ChildrenCount int
}

type StructReply struct {
	*sda.TreeNode
	MessageReply
}
