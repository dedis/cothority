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
	for i, tn := range b.Tree().List()[1:] {
		b.SendTo(tn, &ConnectToAll{i})
	}
	dbg.Lvl3(b.Name(), "Sent Announce to everyone")
	return nil
}

// handleAnnounce receive the announcement from another node
// it reply with an ACK.
func (b *Broadcast) handleConnectToAll(msg struct {
	*sda.TreeNode
	ConnectToAll
}) {
	// Only connect to all nodes that are not the server
	for _, tn := range b.Tree().List()[msg.Index+1:] {
		dbg.Lvl3("Connecting to", tn.Entity.String())
		b.SendTo(tn, &Connected{})
	}
	b.SendTo(msg.TreeNode, &Connected{})
}

// It checks if we have sent an Announce to this treenode (hopefully yes^^)
// if yes it checks if everyone has been ACK'd, if yes, it finishes.
func (b *Broadcast) handleConnected(struct {
	*sda.TreeNode
	Connected
}) {
}

// RegisterOnDone takes a function that will be called once all connections
// are set up.
func (b *Broadcast) RegisterOnDone(fn func()) {
	b.onDoneCb = fn
}

type ConnectToAll struct {
	Index int
}

type Connected struct {
}
