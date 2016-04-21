package sda_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"

	"github.com/satori/go.uuid"
)

// Test setting up of Host
func TestHostNew(t *testing.T) {
	defer dbg.AfterTest(t)
	h1 := sda.NewLocalHost(2000)
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
	defer dbg.AfterTest(t)

	time.Sleep(time.Second)
	dbg.TestOutput(testing.Verbose(), 4)
	h1 := sda.NewLocalHost(2000)
	h2 := sda.NewLocalHost(2001)
	h1.ListenAndBind()
	_, err := h2.Connect(h1.Entity)
	if err != nil {
		t.Fatal("Couldn't Connect()", err)
	}
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	dbg.Lvl3("Finished first connection, starting 2nd")
	h3 := sda.NewLocalHost(2002)
	h3.ListenAndBind()
	c, err := h2.Connect(h3.Entity)
	if err != nil {
		t.Fatal(h2, "Couldn Connect() to", h3)
	}
	dbg.Lvl3("Closing h3")
	err = h3.Close()
	if err != nil {
		// try closing the underlying connection manually and fail
		c.Close()
		t.Fatal("Couldn't Close()", h3)
	}
}

func TestHostClose2(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	defer local.CloseAll()

	_, _, tree := local.GenTree(2, false, true, true)
	dbg.Lvl3(tree.Dump())
	time.Sleep(time.Millisecond * 100)
	dbg.Lvl3("Done")
}

// Test connection of multiple Hosts and sending messages back and forth
// also tests for the counterIO interface that it works well
func TestHostMessaging(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)

	h1, h2 := SetupTwoHosts(t, false)
	bw1 := h1.Tx()
	br2 := h2.Rx()
	msgSimple := &SimpleMessage{3}
	err := h1.SendRaw(h2.Entity, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send from h2 -> h1:", err)
	}
	msg := h2.Receive()
	decoded := testMessageSimple(t, msg)
	if decoded.I != 3 {
		t.Fatal("Received message from h2 -> h1 is wrong")
	}

	written := h1.Tx() - bw1
	read := h2.Rx() - br2
	if written == 0 || read == 0 || written != read {
		t.Logf("Before => bw1 = %d vs br2 = %d", bw1, br2)
		t.Logf("Tx = %d, Rx = %d", written, read)
		t.Logf("h1.Tx() %d vs h2.Rx() %d", h1.Tx(), h2.Rx())
		t.Fatal("Something is wrong with Host.CounterIO")
	}

	h1.Close()
	h2.Close()
}

// Test sending data back and forth using the sendSDAData
func TestHostSendMsgDuplex(t *testing.T) {
	defer dbg.AfterTest(t)
	h1, h2 := SetupTwoHosts(t, false)
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

// Test sending data back and forth using the SendTo
func TestHostSendDuplex(t *testing.T) {
	defer dbg.AfterTest(t)
	h1, h2 := SetupTwoHosts(t, false)
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
	defer dbg.AfterTest(t)
	local := sda.NewLocalTest()
	hosts, el, tree := local.GenTree(2, false, false, false)
	defer local.CloseAll()
	h1 := hosts[0]

	// Add the marshalled version of the tree
	local.AddPendingTreeMarshal(h1, tree.MakeTreeMarshal())
	if _, ok := h1.GetTree(tree.Id); ok {
		t.Fatal("host 1 should not have the tree definition yet.")
	}
	// Now make it check
	local.CheckPendingTreeMarshal(h1, el)
	if _, ok := h1.GetTree(tree.Id); !ok {
		t.Fatal("Host 1 should have the tree definition now.")
	}
}

// Test propagation of peer-lists - both known and unknown
func TestPeerListPropagation(t *testing.T) {
	defer dbg.AfterTest(t)
	local := sda.NewLocalTest()
	hosts, el, _ := local.GenTree(2, false, false, false)
	defer local.CloseAll()
	h1 := hosts[0]
	h2 := hosts[1]
	h2.StartProcessMessages()

	// Check that h2 sends back an empty list if it is unknown
	err := h1.SendRaw(h2.Entity, &sda.RequestEntityList{
		EntityListID: el.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	msg := h1.Receive()
	if msg.MsgType != sda.SendEntityListMessageID {
		t.Fatal("h1 didn't receive EntityList type, but", msg.MsgType)
	}
	if msg.Msg.(sda.EntityList).Id != sda.EntityListID(uuid.Nil) {
		t.Fatal("List should be empty")
	}

	// Now add the list to h2 and try again
	h2.AddEntityList(el)
	err = h1.SendRaw(h2.Entity, &sda.RequestEntityList{EntityListID: el.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	msg = h1.Receive()
	if msg.MsgType != sda.SendEntityListMessageID {
		t.Fatal("h1 didn't receive EntityList type")
	}
	if msg.Msg.(sda.EntityList).Id != el.Id {
		t.Fatal("List should be equal to original list")
	}

	// And test whether it gets stored correctly
	h1.StartProcessMessages()
	err = h1.SendRaw(h2.Entity, &sda.RequestEntityList{EntityListID: el.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	time.Sleep(time.Second)
	list, ok := h1.EntityList(el.Id)
	if !ok {
		t.Fatal("List-id not found")
	}
	if list.Id != el.Id {
		t.Fatal("IDs do not match")
	}
}

// Test propagation of tree - both known and unknown
func TestTreePropagation(t *testing.T) {
	defer dbg.AfterTest(t)
	local := sda.NewLocalTest()
	hosts, el, tree := local.GenTree(2, true, false, false)
	defer local.CloseAll()
	h1 := hosts[0]
	h2 := hosts[1]
	// Suppose both hosts have the list available, but not the tree
	h1.AddEntityList(el)
	h2.AddEntityList(el)
	h2.StartProcessMessages()

	// Check that h2 sends back an empty tree if it is unknown
	err := h1.SendRaw(h2.Entity, &sda.RequestTree{TreeID: tree.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	msg := h1.Receive()
	if msg.MsgType != sda.SendTreeMessageID {
		network.DumpTypes()
		t.Fatal("h1 didn't receive SendTree type:", msg.MsgType)
	}
	if msg.Msg.(sda.TreeMarshal).EntityListID != sda.EntityListID(uuid.Nil) {
		t.Fatal("List should be empty")
	}

	// Now add the list to h2 and try again
	h2.AddTree(tree)
	err = h1.SendRaw(h2.Entity, &sda.RequestTree{TreeID: tree.Id})
	if err != nil {
		t.Fatal("Couldn't send message to h2:", err)
	}
	msg = h1.Receive()
	if msg.MsgType != sda.SendTreeMessageID {
		t.Fatal("h1 didn't receive Tree-type")
	}
	if msg.Msg.(sda.TreeMarshal).TreeId != tree.Id {
		t.Fatal("Tree should be equal to original tree")
	}

	// And test whether it gets stored correctly
	h1.StartProcessMessages()
	err = h1.SendRaw(h2.Entity, &sda.RequestTree{TreeID: tree.Id})
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
}

// Tests both list- and tree-propagation
// basically h1 ask for a tree id
// h2 respond with the tree
// h1 ask for the entitylist (because it dont know)
// h2 respond with the entitylist
func TestListTreePropagation(t *testing.T) {
	defer dbg.AfterTest(t)
	local := sda.NewLocalTest()
	hosts, el, tree := local.GenTree(2, true, true, false)
	defer local.CloseAll()
	h1 := hosts[0]
	h2 := hosts[1]

	// h2 knows the entity list
	h2.AddEntityList(el)
	// and the tree
	h2.AddTree(tree)
	// make the communcation happen
	if err := h1.SendRaw(h2.Entity, &sda.RequestTree{TreeID: tree.Id}); err != nil {
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
		if _, ok := h1.EntityList(el.Id); !ok {
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
}

func TestTokenId(t *testing.T) {
	t1 := &sda.Token{
		EntityListID: sda.EntityListID(uuid.NewV1()),
		TreeID:       sda.TreeID(uuid.NewV1()),
		ProtoID:      sda.ProtocolID(uuid.NewV1()),
		RoundID:      sda.RoundID(uuid.NewV1()),
	}
	id1 := t1.Id()
	t2 := &sda.Token{
		EntityListID: sda.EntityListID(uuid.NewV1()),
		TreeID:       sda.TreeID(uuid.NewV1()),
		ProtoID:      sda.ProtocolID(uuid.NewV1()),
		RoundID:      sda.RoundID(uuid.NewV1()),
	}
	id2 := t2.Id()
	if uuid.Equal(uuid.UUID(id1), uuid.UUID(id2)) {
		t.Fatal("Both token are the same")
	}
	if !uuid.Equal(uuid.UUID(id1), uuid.UUID(t1.Id())) {
		t.Fatal("Twice the Id of the same token should be equal")
	}
	t3 := t1.ChangeTreeNodeID(sda.TreeNodeID(uuid.NewV1()))
	if t1.TreeNodeID.Equals(t3.TreeNodeID) {
		t.Fatal("OtherToken should modify copy")
	}
}

// Test the automatic connection upon request
func TestAutoConnection(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	h1 := sda.NewLocalHost(2000)
	h2 := sda.NewLocalHost(2001)
	h2.ListenAndBind()

	defer h1.Close()
	defer h2.Close()

	err := h1.SendRaw(h2.Entity, &SimpleMessage{12})
	if err != nil {
		t.Fatal("Couldn't send message:", err)
	}

	// Receive the message
	msg := h2.Receive()
	if msg.Msg.(SimpleMessage).I != 12 {
		t.Fatal("Simple message got distorted")
	}
}

func SetupTwoHosts(t *testing.T, h2process bool) (*sda.Host, *sda.Host) {
	hosts := sda.GenLocalHosts(2, true, false)
	if h2process {
		hosts[1].StartProcessMessages()
	}
	return hosts[0], hosts[1]
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

// SimpleMessage is just used to transfer one integer
type SimpleMessage struct {
	I int
}

var SimpleMessageType = network.RegisterMessageType(SimpleMessage{})

func testMessageSimple(t *testing.T, msg network.Message) SimpleMessage {
	if msg.MsgType != SimpleMessageType {
		t.Fatal("Received non SimpleMessage type")
	}
	return msg.Msg.(SimpleMessage)
}
