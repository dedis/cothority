package sda

import (
	"github.com/satori/go.uuid"
	"strconv"
	"testing"
)

var testID = uuid.NewV5(uuid.NamespaceURL, "test")

// Test simple protocol-implementation
// - registration
func TestProtocolRegistration(t *testing.T) {
	if ProtocolExists(testID) {
		t.Fatal("Test should not exist yet")
	}
	ProtocolRegister(testID, NewProtocolTest)
	if !ProtocolExists(testID) {
		t.Fatal("Test should exist now")
	}
}

// Test instantiation of the protocol
func TestProtocolInstantiation(t *testing.T) {
	ProtocolRegister(testID, NewProtocolTest)
	p, err := ProtocolInstantiate(testID, nil, nil, nil)
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
	*Host
	*Tree
	id  uuid.UUID
	tok *Token
}

var currInstanceID int

// NewProtocolTest is used to create a new protocolTest-instance
func NewProtocolTest(n *Host, t *Tree, tok *Token) ProtocolInstance {
	currInstanceID++
	url := "http://dedis.epfl.ch/protocol/test/" + strconv.Itoa(currInstanceID)
	return &ProtocolTest{
		Host: n,
		Tree: t,
		id:   uuid.NewV5(uuid.NamespaceURL, url),
		tok:  tok,
	}
}

func (p *ProtocolTest) Id() uuid.UUID {
	return p.id
}

// Dispatch is used to send the messages further - here everything is
// copied to /dev/null
func (p ProtocolTest) Dispatch(m *SDAData) error {
	return nil
}
