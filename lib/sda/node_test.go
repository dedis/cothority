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

type NodeTestMsg struct {
	I int
}
