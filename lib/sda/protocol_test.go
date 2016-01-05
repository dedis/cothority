package sda_test

import (
	"testing"

	"github.com/dedis/cothority/lib/sda"
)

// Test simple protocol-implementation
// - registration
func TestRegistration(t *testing.T) {
	if sda.ProtocolExists("test") {
		t.Fatal("Test should not exist yet")
	}
	sda.ProtocolRegister("test", NewProtocolTest)
	if !sda.ProtocolExists("test") {
		t.Fatal("Test should exist now")
	}
}

// Test instantiation of the protocol
func TestInstantiation(t *testing.T) {
	sda.ProtocolRegister("test", NewProtocolTest)
	p, err := sda.ProtocolInstantiate("test", nil, nil)
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
	*sda.Node
	*sda.TreePeer
}

// NewProtocolTest is used to create a new protocolTest-instance
func NewProtocolTest(n *sda.Node, t *sda.TreePeer) sda.Protocol {
	return &ProtocolTest{
		Node:     n,
		TreePeer: t,
	}
}

// Dispatch is used to send the messages further - here everything is
// copied to /dev/null
func (p ProtocolTest) Dispatch(m []*sda.Message) error {
	return nil
}
