package manage

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.ProtocolRegisterName("Broadcast", NewBroadcastProtocol)
}

// Broadcast ensures that all nodes are connected to each other. If you need
// a confirmation once everything is set up, you can register a callback-function
// using RegisterOnDone()
type Broadcast struct {
	*sda.TreeNodeInstance

	announceChan chan struct {
		*sda.TreeNode
		Announce
	}

	ackChan chan struct {
		*sda.TreeNode
		ACK
	}

	okChan chan struct {
		*sda.TreeNode
		OK
	}

	// map for all the nodes => state
	listNode map[sda.TreeNodeID]*sda.TreeNode
	// how many peers are connected with me
	ackdNode int
	done     chan bool
	onDoneCb func()
	// how many peers are connected with everyone
	okdNode int
}

// NewBroadcastProtocol returns an initialised protocol for broadcast
func NewBroadcastProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	b := new(Broadcast).init(n)
	// XXX should this start alone ?
	go b.Start()
	return b, nil
}

func (b *Broadcast) init(n *sda.TreeNodeInstance) *Broadcast {
	b.TreeNodeInstance = n

	b.RegisterChannel(&b.ackChan)
	b.RegisterChannel(&b.announceChan)
	b.RegisterChannel(&b.okChan)

	lists := b.Tree().List()
	b.listNode = make(map[sda.TreeNodeID]*sda.TreeNode)
	b.ackdNode = 0
	b.done = make(chan bool, 1)
	for _, tn := range lists {
		if tn.Id.Equals(n.TreeNode().Id) {
			continue
		}
		b.listNode[tn.Id] = tn
	}
	go b.listen()
	return b
}

// NewBroadcastRootProtocol is an abomination that should not exist - will
// be killed with https://github.com/dedis/cothority/pull/325
func NewBroadcastRootProtocol(n *sda.TreeNodeInstance) (*Broadcast, error) {
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
		case msg := <-b.okChan:
			b.handleOk(msg.TreeNode)
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

// RegisterOnDone takes a function that will be called once all connections
// are set up.
func (b *Broadcast) RegisterOnDone(fn func()) {
	b.onDoneCb = fn
}

// Announce is the first message sent to all nodes.
type Announce struct {
}

// ACK is the second message that goes back to the sender.
type ACK struct {
}

// OK is sent from all nodes back to the root.
type OK struct {
}
