package broadcast

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/satori/go.uuid"
)

// Broadcast will just simply broadcast
type Broadcast struct {
	*sda.Node

	announceChan chan struct {
		*sda.TreeNode
		Announce
	}

	ackChan chan struct {
		*sda.TreeNode
		ACK
	}

	// map for all the nodes => state
	listNode map[uuid.UUID]*sda.TreeNode
	ackdNode int
	done     chan bool
	onDoneCb func()
}

func NewBroadcastProtocol(n *sda.Node) (*Broadcast, error) {
	b := new(Broadcast).init(n)
	go b.Start()
	return b, nil
}

func (b *Broadcast) init(n *sda.Node) *Broadcast {
	b.Node = n

	b.RegisterChannel(&b.ackChan)
	b.RegisterChannel(&b.announceChan)

	lists := b.Tree().ListNodes()
	b.listNode = make(map[uuid.UUID]*sda.TreeNode)
	b.ackdNode = 0
	b.done = make(chan bool, 1)
	for _, tn := range lists {
		if uuid.Equal(tn.Id, n.TreeNode().Id) {
			continue
		}
		b.listNode[tn.Id] = tn
	}
	go b.listen()
	return b
}
func NewBroadcastRootProtocol(n *sda.Node) (*Broadcast, error) {
	b := new(Broadcast).init(n)
	// it does not start yet.
	return b, nil
}

func (b *Broadcast) listen() {
	for {
		select {
		case msg := <-b.announceChan:
			b.handleAnnounce(msg.TreeNode)
		case msg := <-b.ackChan:
			b.handleACK(msg.TreeNode)
		case <-b.done:
			return
		}
	}
}

// Start will contact everyone and makes the connections
func (b *Broadcast) Start() error {
	for _, tn := range b.listNode {
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
		dbg.Lvl3(b.Name(), "Received ALL ACK")
		// Yahooo we are done
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
