package sda

var protocols map[string]Protocol

/*
Protocol is the interface that instances have to use in order to be
recognized as protocols
*/
type Protocol interface {
	NewProtocolInstance(n *Node, t *TreePeer)
	Dispatch(m []*Message)
}

/*
ProtocolRegister takes a protocol and registers it under a given name
*/
func ProtocolRegister(name string, protocol Protocol) {

}

/*

 */
func ProtocolExists(name string) bool {
	_, ok := protocols[name]
	return ok
}

/*
ProtocolInstantiate creates a new instance of a protocol given by it's name
*/
func ProtocolInstantiate(name string, n *Node, t *TreePeer) Protocol {
	return nil
}
