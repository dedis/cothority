package sda

/*
Protocol is the interface that instances have to use in order to be
recognized as protocols
*/
type Protocol interface {
	NewProtocol(n *Node, t *TreePeer)
	Dispatch(m []*Message)
}

/*
ProtocolRegister takes a protocol and registers it under a given name
*/
func ProtocolRegister(name string, protocol *Protocol)
