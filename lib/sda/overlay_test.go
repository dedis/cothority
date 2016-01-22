package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"testing"
)

type ProtocolOverlay struct {
	*sda.Node
	done bool
}

func (po *ProtocolOverlay) Start() error {
	// no need to do anything
	return nil
}

func (po *ProtocolOverlay) Dispatch(msgs []*sda.SDAData) error {
	// same here
	return nil
}

func (po *ProtocolOverlay) Release() {
	po.done = true
	// call the Done function
	po.Done()
}

func estOverlayDone(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	// setup
	h1 := newHost("localhost:2000")
	defer h1.Close()
	fn := func(n *sda.Node) sda.ProtocolInstance {
		ps := ProtocolOverlay{
			Node: n,
		}
		return &ps
	}
	el := sda.NewEntityList([]*network.Entity{h1.Entity})
	h1.AddEntityList(el)
	tree, _ := el.GenerateBinaryTree()
	h1.AddTree(tree)
	sda.ProtocolRegisterName("ProtocolOverlay", fn)
	node, err := h1.StartNewNodeName("ProtocolOverlay", tree)
	if err != nil {
		t.Fatal("error starting new node", err)
	}
	po := node.ProtocolInstance().(*ProtocolOverlay)
	// release the resources
	po.Release()
	overlay := h1.Overlay()
	if _, ok := overlay.TokenToNode(po.Token()); ok {
		t.Fatal("Node should not exists after call Done()")
	}
}
