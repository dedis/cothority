package byzcoin

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

// Name can be used from other packages to refer to this protocol.
const Name = "Rollup"

// Reply returns the count of all children.
type Reply struct {
	ChildrenCount int
	Message       string
}
