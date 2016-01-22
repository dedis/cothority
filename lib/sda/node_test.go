package sda_test

import ()

/*
func TestNodeChannel(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	names := genLocalhostPeerNames(10, 2000)
	peerList := genEntityList(tSuite, names)
	// Generate two example topology
	tree, _ := peerList.GenerateBinaryTree()
	dbg.Lvl4("Tree is", tree)

	o := sda.NewOverlay(nil)
	o.RegisterTree(tree)
	n := sda.NewNode(o, &sda.Token{TreeID: tree.Id})
	c := make(chan struct {
		sda.TreeNode
		NodeTestMsg
	}, 1)
	err := n.RegisterChannel(c)
	if err != nil {
		t.Fatal("Couldn't register channel:", err)
	}
	err = n.DispatchChannel(&sda.SDAData{
		Msg:     NodeTestMsg{3},
		MsgType: network.RegisterMessageType(NodeTestMsg{}),
		From: &sda.Token{
			TreeID:     tree.Id,
			TreeNodeID: tree.Root.Id,
		},
	})
	if err != nil {
		t.Fatal("Coulnd't dispatch to channel:", err)
	}
	msg := <-c
	if msg.I != 3 {
		t.Fatal("Message should contain '3'")
	}
}

// Test instantiation of the protocol
func TestProtocolInstantiation(t *testing.T) {
	sda.ProtocolRegister(testID, NewProtocolTest)
	h1, h2 := setupHosts(t, false)
	// Add tree + entitylist
	//el := GenEntityListFrom(h1.Suite(), genLocalhostPeerNames(10, 2000))
	el := sda.NewEntityList([]*network.Entity{h2.Entity, h1.Entity})
	h1.AddEntityList(el)
	tree, _ := el.GenerateBinaryTree()
	h1.AddTree(tree)
	// Then try to instantiate
	tok := &sda.Token{
		ProtocolID:   testID,
		TreeID:       tree.Id,
		EntityListID: tree.EntityList.Id,
	}

	p, err := h1.ProtocolInstantiate(tok, tree.Root)
	if err != nil {
		t.Fatal("Couldn't instantiate test-protocol")
	}
	if p.Dispatch(nil) != nil {
		t.Fatal("Dispatch-method didn't return nil")
	}

	// Try directly StartNewProtocol
	_, err = h1.StartNewProtocol(testID, tree.Id)
	if err != nil {
		t.Fatal("Could not start new protocol")
	}
	if testString == "" {
		t.Fatal("Start() not called")
	}
	h1.Close()
	h2.Close()
}

type NodeTestMsg struct {
	I int
}
*/
