package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"testing"
)

func TestNodeChannel(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	names := genLocalhostPeerNames(10, 2000)
	peerList := genEntityList(tSuite, names)
	// Generate two example topology
	tree, _ := peerList.GenerateBinaryTree()
	dbg.Lvl4("Tree is", tree)
	h := newHost("localhost:2000")
	defer h.Close()

	o := sda.NewOverlay(h)
	o.RegisterTree(tree)
	n, err := sda.NewNode(o, &sda.Token{TreeID: tree.Id})
	c := make(chan struct {
		sda.TreeNode
		NodeTestMsg
	}, 1)
	err = n.RegisterChannel(c)
	if err != nil {
		t.Fatal("Couldn't register channel:", err)
	}
	err = n.DispatchChannel([]*sda.SDAData{&sda.SDAData{
		Msg:     NodeTestMsg{3},
		MsgType: network.RegisterMessageType(NodeTestMsg{}),
		From: &sda.Token{
			TreeID:     tree.Id,
			TreeNodeID: tree.Root.Id,
		}},
	})
	if err != nil {
		t.Fatal("Couldn't dispatch to channel:", err)
	}
	msg := <-c
	if msg.I != 3 {
		t.Fatal("Message should contain '3'")
	}
}

// Test instantiation of Node
func TestNewNode(t *testing.T) {
	sda.ProtocolRegister(testID, NewProtocolTest)
	h1, h2 := setupHosts(t, false)
	// Add tree + entitylist
	//el := GenEntityListFrom(h1.Suite(), genLocalhostPeerNames(10, 2000))
	el := sda.NewEntityList([]*network.Entity{h2.Entity, h1.Entity})
	h1.AddEntityList(el)
	tree, _ := el.GenerateBinaryTree()
	h1.AddTree(tree)

	// Try directly StartNewProtocol
	node, err := h1.StartNewNode(testID, tree)
	if err != nil {
		t.Fatal("Could not start new protocol")
	}
	p := node.ProtocolInstance().(*ProtocolTest)
	if p.Msg != "Start" {
		t.Fatal("Start() not called - msg is:", p.Msg)
	}
	h1.Close()
	h2.Close()
}

type NodeTestMsg struct {
	I int
}
