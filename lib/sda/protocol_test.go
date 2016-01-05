package sda_test

import (
	"github.com/dedis/cothority/lib/sda"
	"testing"
)

// Test simple protocol-implementation
// - registration
// - retrieval
// - instantiation

func TestRegistration(t *testing.T) {
	/*
		if sda.ProtocolExists("test") {
			t.Fatal("Test should not exist yet")
		}
		sda.ProtocolRegister("test", NewProtocolTest)
		if !sda.ProtocolExists("test") {
			t.Fatal("Test should exist now")
		}
	*/
}

type ProtocolTest struct {
	*sda.Node
	*sda.TreePeer
}

func NewProtocolTest(n *sda.Node, t *sda.TreePeer) *ProtocolTest {
	return &ProtocolTest{
		Node:     n,
		TreePeer: t,
	}
}

func (p *ProtocolTest) Dispatch(m []*sda.Message) {

}
