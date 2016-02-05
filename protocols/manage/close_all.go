package manage

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"time"
)

/*
Protocol used to close all connections, starting from the leaf-nodes.
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

type PrepareClose struct{}
type PrepareCloseMsg struct {
	*sda.TreeNode
	PrepareClose
}

type Close struct{}
type CloseMsg struct {
	*sda.TreeNode
	Close
}

func NewCloseAll(n *sda.Node) (sda.ProtocolInstance, error) {
	p := &ProtocolCloseAll{Node: n}
	p.Done = make(chan bool, 1)
	p.RegisterHandler(p.FuncPC)
	p.RegisterHandler(p.FuncC)
	return p, nil
}

func (p *ProtocolCloseAll) FuncPC(pc PrepareCloseMsg) {
	dbg.Lvl3(pc.Entity.Addresses, "sent PrepClose to", p.Entity().Addresses)
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			dbg.Lvl3(p.Entity().Addresses, "sends to", c.Entity.Addresses)
			p.SendTo(c, &PrepareClose{})
		}
	} else {
		p.FuncC(nil)
	}
}

func (p *ProtocolCloseAll) FuncC(c []CloseMsg) {
	if !p.IsRoot() {
		p.SendTo(p.Parent(), &Close{})
	} else {
		dbg.LLvl2("Root received Close")
		p.Done <- true
	}
	time.Sleep(time.Second)
	dbg.Lvl3("Closing host", p.TreeNode().Entity.Addresses)
	err := p.Node.Close()
	if err != nil {
		dbg.Fatal("Couldn't close")
	}
	p.Node.Done()
}

// Starts the protocol
func (p *ProtocolCloseAll) Start() error {
	// Send an empty message
	p.FuncPC(PrepareCloseMsg{TreeNode: p.TreeNode()})
	// Wait till the end
	<-p.Done
	return nil
}
