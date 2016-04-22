package manage

import (
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

/*
Protocol used to close all connections, starting from the leaf-nodes.
It first sends down a message `PrepareClose` through the tree, then
from the leaf-nodes a `Close`-message up to the root. Every node receiving
the `Close`-message will shut down all network communications.

The protocol waits for the `Close`-message to arrive at the root.
*/

func init() {
	network.RegisterMessageType(PrepareClose{})
	network.RegisterMessageType(Close{})
	sda.ProtocolRegisterName("CloseAll", NewCloseAll)
}

// ProtocolCloseAll is the structure used to hold the Done-channel
type ProtocolCloseAll struct {
	*sda.TreeNodeInstance
	// Done receives a 'true' once the protocol is done.
	Done chan bool
}

// PrepareClose is sent down the tree until the nodes
type PrepareClose struct{}

// PrepareCloseMsg is the wrapper for the PrepareClose message
type PrepareCloseMsg struct {
	*sda.TreeNode
	PrepareClose
}

// Close is sent to the parent just before the node shuts down
type Close struct{}

// CloseMsg is the wrapper for the Close message
type CloseMsg struct {
	*sda.TreeNode
	Close
}

// NewCloseAll will create a new protocol
func NewCloseAll(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	p := &ProtocolCloseAll{TreeNodeInstance: n}
	p.Done = make(chan bool, 1)
	if err := p.RegisterHandler(p.FuncPrepareClose); err != nil {
		return nil, err
	}
	if err := p.RegisterHandler(p.FuncClose); err != nil {
		return nil, err
	}
	return p, nil
}

// Start the protocol and waits for the `Close`-message to arrive back at
// the root-node.
func (p *ProtocolCloseAll) Start() error {
	// Send an empty message
	p.FuncPrepareClose(PrepareCloseMsg{TreeNode: p.TreeNode()})
	// Wait till the end
	<-p.Done
	return nil
}

// FuncPrepareClose sends a `PrepareClose`-message down the tree.
func (p *ProtocolCloseAll) FuncPrepareClose(pc PrepareCloseMsg) {
	dbg.Lvl3(pc.Entity.Addresses, "sent PrepClose to", p.Entity().Addresses)
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			err := p.SendTo(c, &PrepareClose{})
			dbg.Lvl3(p.Entity().Addresses, "sends to", c.Entity.Addresses, "(err=", err, ")")
		}
	} else {
		p.FuncClose(nil)
	}
}

// FuncClose is called from the leafs to the parents and up the tree. Everybody
// receiving all `Close`-messages from all children will close down all
// network communication.
func (p *ProtocolCloseAll) FuncClose(c []CloseMsg) {
	if !p.IsRoot() {
		dbg.Lvl3("Sending closeall from", p.Entity().Addresses,
			"to", p.Parent().Entity.Addresses)
		if err := p.SendTo(p.Parent(), &Close{}); err != nil {
			dbg.Error(p.Info(), "couldn't send 'close' tp parent",
				p.Parent(), err)
		}
	} else {
		dbg.Lvl2("Root received Close")
		p.Done <- true
	}
	time.Sleep(time.Second)
	dbg.Lvl3("Closing host", p.Entity().Addresses)
	err := p.TreeNodeInstance.CloseHost()
	if err != nil {
		dbg.Error("Couldn't close:", err)
	}
	p.TreeNodeInstance.Done()
}
