package close_all

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

}

type ProtocolCloseAll struct {
	*sda.Node
	PrepareClose struct {
		sda.TreeNode
		PrepareClose
	}
	Close struct {
		sda.TreeNode
		Close
	}
}

type PrepareClose struct{}

type Close struct{}

func NewCloseAll(n *sda.Node) (sda.ProtocolInstance, error) {
	p := &ProtocolCloseAll{Node: n}
	p.RegisterChannel(&p.PrepareClose)
	p.RegisterChannel(&p.Close)
	return p, nil
}

func (p *ProtocolCloseAll) DispatchChannels() {
	for {
		dbg.Lvl3("waiting for message in", p.Entity().Addresses)
		select {
		case prepare := <-p.PrepareClose:
			dbg.Lvl3("Got preparation to close")
			if len(p.Children()) > 0 {
				for _, c := range p.Children() {
					p.SendTo(c, prepare.PrepareClose)
				}
			} else {
				p.Close <- nil
			}
		case _ := <-p.Close:
			p.SendTo(p.Parent(), &Close{})
			dbg.Lvl3("Closing host")
			err := p.Node.Close()
			if err != nil {
				dbg.Fatal("Couldn't close")
			}
		}
	}
}

// Starts the protocol
func (p *ProtocolCloseAll) Start() error {
	dbg.Lvl3("Starting example")
	return p.HandleAnnounce(MessageAnnounce{"cothority rulez!"})
}

// Dispatch takes the message and decides what function to call
func (p *ProtocolCloseAll) Dispatch(m []*sda.SDAData) error {
	return nil
}
