package messaging

import (
	"errors"

	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

// BroadcastName is the name of this service
var BroadcastName = "Broadcast"

func init() {
	network.RegisterMessage(ContactNodes{})
	network.RegisterMessage(Done{})
	_, err := onet.GlobalProtocolRegister(BroadcastName, NewBroadcastProtocol)
	log.ErrFatal(err)
}

// Broadcast ensures that all nodes are connected to each other. If you need
// a confirmation once everything is set up, you can register a callback-function
// using RegisterOnDone()
type Broadcast struct {
	*onet.TreeNodeInstance
	onDoneCb   func()
	contactRcv int
	doneRcv    int
	tnIndex    int
}

// NewBroadcastProtocol returns a new Broadcast protocol
func NewBroadcastProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	b := &Broadcast{
		TreeNodeInstance: n,
		tnIndex:          -1,
	}
	for i, tn := range n.Tree().List() {
		if tn.ID.Equal(n.TreeNode().ID) {
			b.tnIndex = i
		}
	}
	if b.tnIndex == -1 {
		return nil, errors.New("Didn't find my TreeNode in the Tree")
	}
	// How many done requests we'll send before being done
	b.contactRcv = b.tnIndex
	// How many done requests we'll receive before sending to root
	b.doneRcv = len(b.Tree().List()) - b.tnIndex - 1

	err := n.RegisterHandler(b.handleContactNodes)
	if err != nil {
		return nil, err
	}
	err = n.RegisterHandler(b.handleDone)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Start will contact everyone and make the connections
func (b *Broadcast) Start() error {
	b.doneRcv *= 2
	log.Lvl3(b.Name(), "Sending announce and waiting for", b.doneRcv, "confirmations")
	b.SendTo(b.Root(), &ContactNodes{})
	return nil
}

// handleAnnounce receive the announcement from another node
// it reply with an ACK.
func (b *Broadcast) handleContactNodes(msg struct {
	*onet.TreeNode
	ContactNodes
}) error {
	log.Lvl3(b.ServerIdentity(), "Received contact message from", msg.TreeNode.ServerIdentity)
	err := b.SendTo(msg.TreeNode, &Done{})
	if err != nil {
		return err
	}
	if msg.TreeNode.ID.Equal(b.Root().ID) {
		// Connect to all nodes that are later in the TreeNodeList.
		for _, tn := range b.Tree().List()[b.tnIndex+1:] {
			log.Lvl3("Contacting", tn.String())
			err := b.SendTo(tn, &ContactNodes{})
			if err != nil {
				return err
			}
		}
	}
	b.contactRcv--
	return b.verifyDone()
}

func (b *Broadcast) verifyDone() error {
	if b.contactRcv+b.doneRcv == 0 {
		defer b.Done()
		err := b.SendTo(b.Root(), &Done{})
		if err != nil {
			return err
		}
		if b.onDoneCb != nil {
			b.onDoneCb()
		}
	}
	return nil
}

// Every node being contacted sends back a Done to the root which has
// to count to decide if all is done
func (b *Broadcast) handleDone(msg struct {
	*onet.TreeNode
	Done
}) error {
	b.doneRcv--
	return b.verifyDone()
}

// RegisterOnDone takes a function that will be called once all connections
// are set up.
func (b *Broadcast) RegisterOnDone(fn func()) {
	b.onDoneCb = fn
}

// ContactNodes is sent from the root to ALL other nodes
type ContactNodes struct{}

// Done is sent back to root once everybody has been contacted
type Done struct{}
