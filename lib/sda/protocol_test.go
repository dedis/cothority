package sda

import (
	"strconv"
	"testing"
)

// Test simple protocol-implementation
// - registration
func TestRegistration(t *testing.T) {
	if ProtocolExists("test") {
		t.Fatal("Test should not exist yet")
	}
	ProtocolRegister("test", NewProtocolTest)
	if !ProtocolExists("test") {
		t.Fatal("Test should exist now")
	}
}

// Test instantiation of the protocol
func TestInstantiation(t *testing.T) {
	ProtocolRegister("test", NewProtocolTest)
	p, err := ProtocolInstantiate("test", nil, nil)
	if err != nil {
		t.Fatal("Couldn't instantiate test-protocol")
	}
	if p.Dispatch(nil) != nil {
		t.Fatal("Dispatch-method didn't return nil")
	}
}

// ProtocolTest is the most simple protocol to be implemented, ignoring
// everything it receives.
type ProtocolTest struct {
	*Node
	*Tree
	ID string
}

var currInstanceID int

// NewProtocolTest is used to create a new protocolTest-instance
func NewProtocolTest(n *Node, t *Tree) ProtocolInstance {
	currInstanceID++
	return &ProtocolTest{
		Node: n,
		Tree: t,
		ID:   strconv.Itoa(currInstanceID),
	}
}

// Dispatch is used to send the messages further - here everything is
// copied to /dev/null
func (p ProtocolTest) Dispatch(m *SDAMessage) error {
	return nil
}

func (p *ProtocolTest) Id() UUID {
	return UUID(p.ID)
}
