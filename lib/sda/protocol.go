package sda

import "errors"

/*
NewProtocol is the function-signature needed to instantiate a new protocol
*/
type NewProtocol func(*Node, *TreePeer) Protocol

// protocols holds a map of all available protocols
var protocols map[string]NewProtocol

/*
Protocol is the interface that instances have to use in order to be
recognized as protocols
*/
type Protocol interface {
	Dispatch(m []*Message) error
}

/*
ProtocolRegister takes a protocol and registers it under a given name
*/
func ProtocolRegister(name string, protocol NewProtocol) {
	if protocols == nil {
		protocols = make(map[string]NewProtocol)
	}
	protocols[name] = protocol
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
func ProtocolInstantiate(name string, n *Node, t *TreePeer) (Protocol, error) {
	p, ok := protocols[name]
	if !ok {
		return nil, errors.New("Protocol doesn't exist")
	}
	return p(n, t), nil
}
