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
	network.RegisterMessageType(PrepareCount{})
	network.RegisterMessageType(Count{})
	sda.ProtocolRegisterName("Count", NewCount)
}

type ProtocolCount struct {
	*sda.Node
	Count chan int
}

type PrepareCount struct{}
type PrepareCountMsg struct {
	*sda.TreeNode
	PrepareCount
}

type Count struct {
	Children int
}
type CountMsg struct {
	*sda.TreeNode
	Count
}

func NewCount(n *sda.Node) (sda.ProtocolInstance, error) {
	p := &ProtocolCount{Node: n}
	p.Count = make(chan int, 1)
	p.RegisterHandler(p.FuncPC)
	p.RegisterHandler(p.FuncC)
	return p, nil
}

func (p *ProtocolCount) FuncPC(pc PrepareCountMsg) {
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			dbg.Lvl3("Sending to", c.Entity.Addresses)
			p.SendTo(c, &PrepareCount{})
		}
	} else {
		p.FuncC(nil)
	}
}

func (p *ProtocolCount) FuncC(c []CountMsg) {
	count := 1
	for _, c := range c {
		count += c.Count.Children
	}
	if !p.IsRoot() {
		p.SendTo(p.Parent(), &Count{count})
	} else {
		p.Count <- count
	}
}

// Starts the protocol
func (p *ProtocolCount) Start() error {
	// Send an empty message
	p.FuncPC(PrepareCountMsg{})
	return nil
}
