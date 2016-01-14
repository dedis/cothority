package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/satori/go.uuid"
	"strconv"
	"testing"
)

var testID = uuid.NewV5(uuid.NamespaceURL, "test")

// Test simple protocol-implementation
// - registration
func TestProtocolRegistration(t *testing.T) {
	if sda.ProtocolExists(testID) {
		t.Fatal("Test should not exist yet")
	}
	sda.ProtocolRegister(testID, NewProtocolTest)
	if !sda.ProtocolExists(testID) {
		t.Fatal("Test should exist now")
	}
}

var testString = ""

// Test instantiation of the protocol
func TestProtocolInstantiation(t *testing.T) {
	sda.ProtocolRegister(testID, NewProtocolTest)
	h1, h2 := setupHosts(t, false)
	// Add tree + entitylist
	el := GenEntityList(h1.Suite(), genLocalhostPeerNames(10, 2000))
	h1.AddEntityList(el)
	tree, _ := GenerateTreeFromEntityList(el)
	h1.AddTree(tree)
	// Then try to instantiate
	tok := &sda.Token{
		ProtocolID:   testID,
		TreeID:       tree.Id,
		EntityListID: tree.IdList.Id,
	}

	p, err := h1.ProtocolInstantiate(tok)
	if err != nil {
		t.Fatal("Couldn't instantiate test-protocol")
	}
	if p.Dispatch(nil) != nil {
		t.Fatal("Dispatch-method didn't return nil")
	}

	// Try directly StartNewProtocol
	err = h1.StartNewProtocol(testID, tree.Id)
	if err != nil {
		t.Fatal("Could not start new protocol")
	}
	if testString == "" {
		t.Fatal("Start() not called")
	}
	h1.Close()
	h2.Close()
}

// ProtocolTest is the most simple protocol to be implemented, ignoring
// everything it receives.
type ProtocolTest struct {
	*sda.Host
	*sda.Tree
	id  uuid.UUID
	tok *sda.Token
}

var currInstanceID int

// NewProtocolTest is used to create a new protocolTest-instance
func NewProtocolTest(n *sda.Host, t *sda.Tree, tok *sda.Token) sda.ProtocolInstance {
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
func (p *ProtocolTest) Dispatch(m *sda.SDAData) error {
	dbg.Lvl2("PRotocolTest.Dispatch()")
	return nil
}

func (p *ProtocolTest) Start() {
	dbg.Lvl2("ProtocolTest.Start()")
	testString = "ProtocolTest"
}
