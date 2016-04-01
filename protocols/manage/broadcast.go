// Package manage implements a protocol which sends a message to all nodes so
// that the connections are set up
package manage

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.ProtocolRegisterName("Broadcast", NewBroadcastProtocol)
}

// Broadcast will just simply broadcast
type Broadcast struct {
	*sda.Node
	onDoneCb func()
}

// NewBroadcastProtocol returns a new Broadcast protocol
func NewBroadcastProtocol(n *sda.Node) (sda.ProtocolInstance, error) {
	b := &Broadcast{n}

	return b
}

// Start will contact everyone and makes the connections
func (b *Broadcast) Start() error {
	for _, tn := range b.Tree().ListNodes() {
		b.SendTo(tn, &Announce{})
	}
	dbg.Lvl3(b.Name(), "Sent Announce to everyone")
	return nil
}

// handleAnnounce receive the announcement from another node
// it reply with an ACK.
func (b *Broadcast) handleAnnounce(tn *sda.TreeNode) {
	b.SendTo(tn, &ACK{})
}

// It checks if we have sent an Announce to this treenode (hopefully yes^^)
// if yes it checks if everyone has been ACK'd, if yes, it finishes.
func (b *Broadcast) handleACK(tn *sda.TreeNode) {
	if _, ok := b.listNode[tn.Id]; !ok {
		dbg.Error(b.Name(), "Broadcast Received ACK from unknown treenode")
	}

	b.ackdNode++
	if b.ackdNode == len(b.listNode) {
		if !b.IsRoot() {
			b.SendTo(b.Tree().Root, &OK{})
			dbg.Lvl3(b.Name(), "Received ALL ACK (notified the root)")
		}
	}
}

func (b *Broadcast) handleOk(tn *sda.TreeNode) {
	if _, ok := b.listNode[tn.Id]; !ok {
		dbg.Error(b.Name(), "Broadcast Received ACK from unknown treenode")
	}

	b.okdNode++
	dbg.Lvl2(b.Name(), "Received OK with ackNode=", b.ackdNode, " and okdNode=", b.okdNode)
	if b.ackdNode == len(b.listNode) && b.okdNode == len(b.listNode) {
		// Yahooo we are done
		dbg.Lvl3(b.Name(), " Knows EVERYONE is connected to EVERYONE")
		b.done <- true
		if b.onDoneCb != nil {
			b.onDoneCb()
		}
	}

}

func (b *Broadcast) RegisterOnDone(fn func()) {
	b.onDoneCb = fn
}

type Announce struct {
}

type ACK struct {
}

// OK means I am connected with everyone and I tell you this.
type OK struct {
}
