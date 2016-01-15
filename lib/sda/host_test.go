// better not get sda_test, cannot access unexported fields
package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/random"
	"github.com/satori/go.uuid"
	"strconv"
	"testing"
	"time"
)

var suite abstract.Suite = network.Suite

// Test setting up of Host
func TestHostNew(t *testing.T) {
	h1 := newHost("localhost:2000", suite)
	if h1 == nil {
		t.Fatal("Couldn't setup a Host")
	}
	err := h1.Close()
	if err != nil {
		t.Fatal("Couldn't close", err)
	}
}

// Test closing and opening of Host on same address
func TestHostClose(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	h1 := newHost("localhost:2000", suite)
	h2 := newHost("localhost:2001", suite)
	h1.Listen()
	h2.Connect(h1.Entity)
	err := h1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	dbg.Lvl3("Finished first connection, starting 2nd")
	h1 = newHost("localhost:2002", suite)
	h1.Listen()
	if err != nil {
		t.Fatal("Couldn't re-open listener")
	}
	dbg.Lvl3("Closing h1")
	h1.Close()
}

// Test connection of multiple Hosts and sending messages back and forth
func TestHostMessaging(t *testing.T) {
	h1, h2 := setupHosts(t, false)
	msgSimple := &SimpleMessage{3}
	err := h1.SendSDAData(h2.Entity, &sda.SDAData{Msg: msgSimple})
	if err != nil {
		t.Fatal("Couldn't send from h2 -> h1:", err)
	}
	msg := h2.Receive()
	decoded := testMessageSimple(t, msg)
	if decoded.I != 3 {
		t.Fatal("Received message from h2 -> h1 is wrong")
	}

	h1.Close()
	h2.Close()
}

// Test parsing of incoming packets with regard to its double-included
// data-structure
func TestHostIncomingMessage(t *testing.T) {
	h1, h2 := setupHosts(t, false)
	msgSimple := &SimpleMessage{10}
	err := h1.SendSDAData(h2.Entity, &sda.SDAData{Msg: msgSimple})
	if err != nil {
		t.Fatal("Couldn't send message:", err)
	}

	msg := h2.Receive()
	decoded := testMessageSimple(t, msg)
	if decoded.I != 10 {
		t.Fatal("Wrong value")
	}

	h1.Close()
	h2.Close()
}

// Test sending data back and forth using the sendSDAData
func TestHostSendMsgDuplex(t *testing.T) {
	h1, h2 := setupHosts(t, false)
	msgSimple := &SimpleMessage{5}
	err := h1.SendSDAData(h2.Entity, &sda.SDAData{Msg: msgSimple})
	if err != nil {
		t.Fatal("Couldn't send message from h1 to h2", err)
	}
	msg := h2.Receive()
	dbg.Lvl2("Received msg h1 -> h2", msg)

	err = h2.SendSDAData(h1.Entity, &sda.SDAData{Msg: msgSimple})
	if err != nil {
		t.Fatal("Couldn't send message from h2 to h1", err)
	}
	msg = h1.Receive()
	dbg.Lvl2("Received msg h2 -> h1", msg)

	h1.Close()
	h2.Close()
}

// Test sending data back and forth using the SendTo
func TestHostSendDuplex(t *testing.T) {
	h1, h2 := setupHosts(t, false)
	msgSimple := &SimpleMessage{5}
	err := h1.SendRaw(h2.Entity, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message from h1 to h2", err)
	}
	msg := h2.Receive()
	dbg.Lvl2("Received msg h1 -> h2", msg)

	err = h2.SendRaw(h1.Entity, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message from h2 to h1", err)
	}
	msg = h1.Receive()
	dbg.Lvl2("Received msg h2 -> h1", msg)

	h1.Close()
	h2.Close()
}

// Test when a peer receives a New EntityList, it can create the trees that are
// waiting on this specific entitiy list, to be constructed.
func TestPeerPendingTreeMarshal(t *testing.T) {
	h1, h2 := setupHosts(t, false)
	//el := GenEntityList(h1.Suite(), genLocalhostPeerNames(10, 2000))
	el := GenEntityListFromHost(h2, h1)
	tree, _ := el.GenerateBinaryTree()

	// Add the marshalled version of the tree
	h1.AddPendingTreeMarshal(tree.MakeTreeMarshal())
	if _, ok := h1.GetTree(tree.Id); ok {
		t.Fatal("host 1 should not have the tree definition yet.")
	}
	// Now make it check
	h1.CheckPendingTreeMarshal(el)
	if _, ok := h1.GetTree(tree.Id); !ok {
		t.Fatal("Host 1 should have the tree definition now.")
	}
	h1.Close()
	h2.Close()
}

// Test propagation of peer-lists - both known and unknown
func TestPeerListPropagation(t *testing.T) {
	h1, h2 := setupHosts(t, true)
	//il1 := GenEntityList(h1.Suite(), genLocalhostPeerNames(10, 2000))
	el1 := GenEntityListFromHost(h2, h1)
	// Check that h2 sends back an empty list if it is unknown
	err := h1.SendRaw(h2.Entity, &sda.RequestEntityList{el1.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	msg := h1.Receive()
	if msg.MsgType != sda.SendEntityListMessage {
		t.Fatal("h1 didn't receive EntityList type, but", msg.MsgType)
	}
	if msg.Msg.(sda.EntityList).Id != uuid.Nil {
		t.Fatal("List should be empty")
	}

	// Now add the list to h2 and try again
	h2.AddEntityList(el1)
	err = h1.SendRaw(h2.Entity, &sda.RequestEntityList{el1.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	msg = h1.Receive()
	if msg.MsgType != sda.SendEntityListMessage {
		t.Fatal("h1 didn't receive EntityList type")
	}
	if msg.Msg.(sda.EntityList).Id != el1.Id {
		t.Fatal("List should be equal to original list")
	}

	// And test whether it gets stored correctly
	go h1.ProcessMessages()
	err = h1.SendRaw(h2.Entity, &sda.RequestEntityList{el1.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	time.Sleep(time.Second)
	list, ok := h1.GetEntityList(el1.Id)
	if !ok {
		t.Fatal("List-id not found")
	}
	if list.Id != el1.Id {
		t.Fatal("IDs do not match")
	}
	h1.Close()
	h2.Close()
}

// Test propagation of tree - both known and unknown
func TestTreePropagation(t *testing.T) {
	h1, h2 := setupHosts(t, true)
	//il1 := GenEntityList(h1.Suite(), genLocalhostPeerNames(10, 2000))
	el1 := GenEntityListFromHost(h2, h1)
	// Suppose both hosts have the list available, but not the tree
	h1.AddEntityList(el1)
	h2.AddEntityList(el1)
	tree, _ := el1.GenerateBinaryTree()

	// Check that h2 sends back an empty tree if it is unknown
	err := h1.SendRaw(h2.Entity, &sda.RequestTree{tree.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	msg := h1.Receive()
	if msg.MsgType != sda.SendTreeMessage {
		network.DumpTypes()
		t.Fatal("h1 didn't receive SendTree type:", msg.MsgType)
	}
	if msg.Msg.(sda.TreeMarshal).EntityId != uuid.Nil {
		t.Fatal("List should be empty")
	}

	// Now add the list to h2 and try again
	h2.AddTree(tree)
	err = h1.SendRaw(h2.Entity, &sda.RequestTree{tree.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	msg = h1.Receive()
	if msg.MsgType != sda.SendTreeMessage {
		t.Fatal("h1 didn't receive Tree-type")
	}
	if msg.Msg.(sda.TreeMarshal).NodeId != tree.Id {
		t.Fatal("Tree should be equal to original tree")
	}

	// And test whether it gets stored correctly
	go h1.ProcessMessages()
	err = h1.SendRaw(h2.Entity, &sda.RequestTree{tree.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	time.Sleep(time.Second)
	tree2, ok := h1.GetTree(tree.Id)
	if !ok {
		t.Fatal("List-id not found")
	}
	if !tree.Equal(tree2) {
		t.Fatal("Trees do not match")
	}
	h1.Close()
	h2.Close()
}

// Tests both list- and tree-propagation
// basically h1 ask for a tree id
// h2 respond with the tree
// h1 ask for the entitylist (because it dont know)
// h2 respond with the entitylist
func TestListTreePropagation(t *testing.T) {
	h1, h2 := setupHosts(t, true)
	//el := GenEntityList(h1.Suite(), genLocalhostPeerNames(10, 2000))
	el := GenEntityListFromHost(h2, h1)
	tree, _ := el.GenerateBinaryTree()
	// h2 knows the entity list
	h2.AddEntityList(el)
	// and the tree
	h2.AddTree(tree)
	// make host1 listen, so it will process messages as host2 is sending
	// it is supposed to automatically ask for the entitylist
	go h1.ProcessMessages()
	// make the communcation happen
	if err := h1.SendRaw(h2.Entity, &sda.RequestTree{tree.Id}); err != nil {
		t.Fatal("Could not send tree request to host2", err)
	}

	var tryTree int
	var tryEntity int
	var found bool
	for tryTree < 5 || tryEntity < 5 {
		// Sleep a bit
		time.Sleep(100 * time.Millisecond)
		// then look if we have both the tree and the entity list
		if _, ok := h1.GetTree(tree.Id); !ok {
			tryTree++
			continue
		}
		// We got the tree that's already something, now do we get the entity
		// list
		if _, ok := h1.GetEntityList(el.Id); !ok {
			tryEntity++
			continue
		}
		// we got both ! yay
		found = true
		break
	}
	if !found {
		t.Fatal("Did not get the tree + entityList from host2")
	}
	h1.Close()
	h2.Close()

}

func TestTokenId(t *testing.T) {
	t1 := &sda.Token{
		EntityListID: uuid.NewV1(),
		TreeID:       uuid.NewV1(),
		ProtocolID:   uuid.NewV1(),
		RoundID:      uuid.NewV1(),
	}
	id1 := t1.Id()
	t2 := &sda.Token{
		EntityListID: uuid.NewV1(),
		TreeID:       uuid.NewV1(),
		ProtocolID:   uuid.NewV1(),
		RoundID:      uuid.NewV1(),
	}
	id2 := t2.Id()
	if uuid.Equal(id1, id2) {
		t.Fatal("Both token are the same")
	}
	if !uuid.Equal(id1, t1.Id()) {
		t.Fatal("Twice the Id of the same token should be equal")
	}
	t3 := t1.OtherToken(&sda.TreeNode{Id: uuid.NewV1()})
	if uuid.Equal(t1.TreeNodeID, t3.TreeNodeID) {
		t.Fatal("OtherToken should modify copy")
	}
}

// Test instantiation of ProtocolInstances

// Test access of actual peer that received the message
// - corner-case: accessing parent/children with multiple instances of the same peer
// in the graph - ProtocolID + GraphID + InstanceID is not enough
// XXX ???

// Test complete parsing of new incoming packet
// - Test if it is SDAMessage
// - reject if unknown ProtocolID
// - setting up of graph and Hostlist
// - instantiating ProtocolInstance

// privPub creates a private/public key pair
func privPub(s abstract.Suite) (abstract.Secret, abstract.Point) {
	keypair := &config.KeyPair{}
	keypair.Gen(s, random.Stream)
	return keypair.Secret, keypair.Public
}

func newHost(address string, s abstract.Suite) *sda.Host {
	priv, pub := privPub(s)
	id := network.NewEntity(pub, address)
	return sda.NewHost(id, priv)
}

// Creates two hosts on the local interfaces,
func setupHosts(t *testing.T, h2process bool) (*sda.Host, *sda.Host) {
	dbg.TestOutput(testing.Verbose(), 4)
	h1 := newHost("localhost:2000", suite)
	// make the second peer as the server
	h2 := newHost("localhost:2001", suite)
	h2.Listen()
	// make it process messages
	if h2process {
		go h2.ProcessMessages()
	}
	_, err := h1.Connect(h2.Entity)
	if err != nil {
		t.Fatal(err)
	}
	return h1, h2
}

// GenHosts will create n hosts with the first one being connected to each of
// the other node
func GenHosts(t *testing.T, n int) []*sda.Host {
	var hosts []*sda.Host
	for i := 0; i < n; i++ {
		hosts = append(hosts, newHost("localhost:"+strconv.Itoa(2000+i*10), suite))
	}
	root := hosts[0]
	root.Listen()
	go root.ProcessMessages()
	for i := 1; i < n; i++ {
		hosts[i].Listen()
		go hosts[i].ProcessMessages()
		if _, err := hosts[i].Connect(root.Entity); err != nil {
			t.Fatal("Could not connect hosts")
		}
	}
	return hosts
}

// SimpleMessage is just used to transfer one integer
type SimpleMessage struct {
	I int
}

var SimpleMessageType = network.RegisterMessageType(SimpleMessage{})

func testMessageSimple(t *testing.T, msg network.ApplicationMessage) SimpleMessage {
	if msg.MsgType != sda.SDADataMessage {
		t.Fatal("Wrong message type received:", msg.MsgType)
	}
	sda := msg.Msg.(sda.SDAData)
	if sda.MsgType != SimpleMessageType {
		t.Fatal("Couldn't pass simple message")
	}
	return sda.Msg.(SimpleMessage)
}

func GenEntityListFromHost(hosts ...*sda.Host) *sda.EntityList {
	var entities []*network.Entity
	for i := range hosts {
		entities = append(entities, hosts[i].Entity)
	}
	return sda.NewEntityList(entities)
}
