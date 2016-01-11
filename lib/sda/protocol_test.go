package sda

import (
	"github.com/satori/go.uuid"
	"strconv"
	"testing"
)

// Test simple protocol-implementation
// - registration
func TestRegistration(t *testing.T) {
	if ProtocolExists(ProtocolTestUUID) {
		t.Fatal("Test should not exist yet")
	}
	ProtocolRegister(ProtocolTestUUID, NewProtocolTest)
	if !ProtocolExists(ProtocolTestUUID) {
		t.Fatal("Test should exist now")
	}
}

// Test instantiation of the protocol
func TestInstantiation(t *testing.T) {
	ProtocolRegister(ProtocolTestUUID, NewProtocolTest)
	p, err := ProtocolInstantiate(ProtocolTestUUID, nil, nil)
	if err != nil {
		t.Fatal("Couldn't instantiate test-protocol")
	}
	if p.Dispatch(nil) != nil {
		t.Fatal("Dispatch-method didn't return nil")
	}
}

var ProtocolTestUUID = uuid.NewV4()

// ProtocolTest is the most simple protocol to be implemented, ignoring
// everything it receives.
type ProtocolTest struct {
	*Host
	*Tree
	ID uuid.UUID
}

var currInstanceID int

// NewProtocolTest is used to create a new protocolTest-instance
func NewProtocolTest(n *Host, t *Tree) ProtocolInstance {
	currInstanceID++
	uid, _ := uuid.FromString(strconv.Itoa(currInstanceID))
	return &ProtocolTest{
		Host: n,
		Tree: t,
		ID:   uid,
	}
}

// Dispatch is used to send the messages further - here everything is
// copied to /dev/null
func (p ProtocolTest) Dispatch(m *SDAMessage) error {
	return nil
}

func (p *ProtocolTest) Id() uuid.UUID {
	return p.ID
}
