package sda_test

import (
	"github.com/dedis/cothority/lib/sda"
	"testing"
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

// - instantiation
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

type ProtocolTest struct {
	*sda.Node
	*sda.TreePeer
}

func NewProtocolTest(n *sda.Node, t *sda.TreePeer) sda.Protocol {
	return &ProtocolTest{
		Node:     n,
		TreePeer: t,
	}
}

func (p ProtocolTest) Dispatch(m []*sda.Message) error {
	return nil
}
