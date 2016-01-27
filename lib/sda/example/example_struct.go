package example

// MessageAnnounce is used to pass a message to all children
type MessageAnnounce struct {
	Message string
}

// MessageReply returns the count of all children
type MessageReply struct {
	ChildrenCount int
}
