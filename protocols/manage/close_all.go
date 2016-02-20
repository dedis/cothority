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

type ProtocolCloseAll struct {
	*sda.Node
	Done chan bool
}

type PrepareClose struct {
	NonEmpty string
}
type PrepareCloseMsg struct {
	*sda.TreeNode
	PrepareClose
}

type Close struct {
	NonEmpty string
}
type CloseMsg struct {
	*sda.TreeNode
	Close
}

// NewCloseAll will create a new protocol
func NewCloseAll(n *sda.Node) (sda.ProtocolInstance, error) {
	p := &ProtocolCloseAll{Node: n}
	p.Done = make(chan bool, 1)
	p.RegisterHandler(p.FuncPrepareClose)
	p.RegisterHandler(p.FuncClose)
	return p, nil
}

// FuncPrepareClose sends a `PrepareClose`-message down the tree.
func (p *ProtocolCloseAll) FuncPrepareClose(pc PrepareCloseMsg) {
	dbg.Lvl3(pc.Entity.Addresses, "sent PrepClose to", p.Entity().Addresses)
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			err := p.SendTo(c, &PrepareClose{"PrepClose"})
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
		p.SendTo(p.Parent(), &Close{"Close"})
	} else {
		dbg.Lvl2("Root received Close")
		p.Done <- true
	}
	time.Sleep(time.Second)
	dbg.Lvl3("Closing host", p.Entity().Addresses)
	err := p.Node.Close()
	if err != nil {
		dbg.Error("Couldn't close:", err)
	}
	p.Node.Done()
}

// Starts the protocol and waits for the `Close`-message to arrive back at
// the root-node.
func (p *ProtocolCloseAll) Start() error {
	// Send an empty message
	p.FuncPrepareClose(PrepareCloseMsg{TreeNode: p.TreeNode()})
	// Wait till the end
	<-p.Done
	return nil
}
