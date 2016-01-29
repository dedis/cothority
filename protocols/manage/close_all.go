package manage

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
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
	Done         chan bool
	PrepareClose chan struct {
		*sda.TreeNode
		PrepareClose
	}
	Close chan struct {
		*sda.TreeNode
		Close
	}
}

type PrepareClose struct{}

type Close struct{}

func NewCloseAll(n *sda.Node) (sda.ProtocolInstance, error) {
	p := &ProtocolCloseAll{Node: n}
	p.Done = make(chan bool, 1)
	p.RegisterChannel(&p.PrepareClose)
	p.RegisterChannel(&p.Close)
	go p.DispatchChannels()
	return p, nil
}

func (p *ProtocolCloseAll) DispatchChannels() {
	for {
		dbg.Lvl3("waiting for message in", p.Entity().Addresses)
		select {
		case _ = <-p.PrepareClose:
			p.FuncPC()
		case _ = <-p.Close:
			p.FuncC()
		}
	}
}

func (p *ProtocolCloseAll) FuncPC() {
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			dbg.Lvl3("Sending to", c.Entity.Addresses)
			p.SendTo(c, &PrepareClose{})
		}
	} else {
		p.FuncC()
	}
}

func (p *ProtocolCloseAll) FuncC() {
	if !p.IsRoot() {
		p.SendTo(p.Parent(), &Close{})
	} else {
		p.Done <- true
	}
	dbg.Lvl3("Closing host")
	err := p.Node.Close()
	if err != nil {
		dbg.Fatal("Couldn't close")
	}
}

// Starts the protocol
func (p *ProtocolCloseAll) Start() error {
	// Send an empty message
	p.FuncPC()
	// Wait till the end
	<-p.Done
	return nil
}

// Dispatch takes the message and decides what function to call
func (p *ProtocolCloseAll) Dispatch(m []*sda.SDAData) error {
	return nil
}
