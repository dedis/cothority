package sda

import (
	"errors"
)

/*
NewProtocol is the function-signature needed to instantiate a new protocol
*/
type NewProtocol func(*Node) ProtocolInstance
type ProtocolID string

// protocols holds a map of all available protocols and how to create an
// instance of it
var protocols map[ProtocolID]NewProtocol

/*
Protocol is the interface that instances have to use in order to be
recognized as protocols
*/
type ProtocolInstance interface {
	// A protocol isntance should be able to dispatch its own message internally
	Dispatch(m *SDAMessage) error
	// and give a unique identifier like a GUID
	Id() InstanceID
}
type InstanceID string

/*
ProtocolInstantiate creates a new instance of a protocol given by it's name
*/
func ProtocolInstantiate(protoID ProtocolID, n *Node) (ProtocolInstance, error) {
	p, ok := protocols[protoID]
	if !ok {
		return nil, errors.New("Protocol doesn't exist")
	}
	return p(n), nil
}

/*
ProtocolRegister takes a protocol and registers it under a given ID
*/
func ProtocolRegister(ID ProtocolID, protocol NewProtocol) {
	protocols[ID] = protocol
}

/*

 */
func ProtocolExists(ID ProtocolID) bool {
	_, ok := protocols[ID]
	return ok
}

func init() {
	protocols = make(map[ProtocolID]NewProtocol)
}
