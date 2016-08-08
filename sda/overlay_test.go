package sda

import (
	"testing"

	"github.com/dedis/cothority/network"
)

type ProtocolOverlay struct {
	*TreeNodeInstance
	done bool
}

func (po *ProtocolOverlay) Start() error {
	// no need to do anything
	return nil
}

func (po *ProtocolOverlay) Dispatch() error {
	return nil
}

func (po *ProtocolOverlay) Release() {
	// call the Done function
	po.Done()
}

func TestOverlayDone(t *testing.T) {
	// setup
	h1 := NewLocalHost(2000)
	defer h1.Close()
	fn := func(n *TreeNodeInstance) (ProtocolInstance, error) {
		ps := ProtocolOverlay{
			TreeNodeInstance: n,
		}
		return &ps, nil
	}
	el := NewRoster([]*network.ServerIdentity{h1.ServerIdentity})
	h1.AddRoster(el)
	tree := el.GenerateBinaryTree()
	h1.AddTree(tree)
	ProtocolRegisterName("ProtocolOverlay", fn)
	p, err := h1.CreateProtocol("ProtocolOverlay", tree)
	if err != nil {
		t.Fatal("error starting new node", err)
	}
	po := p.(*ProtocolOverlay)
	// release the resources
	var count int
	po.OnDoneCallback(func() bool {
		count++
		if count >= 2 {
			return true
		}
		return false
	})
	po.Release()
	overlay := h1.Overlay()
	if _, ok := overlay.TokenToNode(po.Token()); !ok {
		t.Fatal("Node should exists after first call Done()")
	}
	po.Release()
	if _, ok := overlay.TokenToNode(po.Token()); ok {
		t.Fatal("Node should NOT exists after call Done()")
	}
}
