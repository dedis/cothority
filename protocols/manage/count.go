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
	network.RegisterMessageType(PrepareCount{})
	network.RegisterMessageType(Count{})
	sda.ProtocolRegisterName("Count", NewCount)
}

type ProtocolCount struct {
	*sda.Node
	Count            chan int
	Quit             chan bool
	PrepareCountChan chan struct {
		*sda.TreeNode
		PrepareCount
	}
	CountChan chan []CountMsg
}

type PrepareCount struct{}

type Count struct {
	Children int
}
type CountMsg struct {
	*sda.TreeNode
	Count
}

func NewCount(n *sda.Node) (sda.ProtocolInstance, error) {
	p := &ProtocolCount{
		Node: n,
		Quit: make(chan bool),
	}
	p.Count = make(chan int, 1)
	p.RegisterChannel(&p.CountChan)
	p.RegisterChannel(&p.PrepareCountChan)
	return p, nil
}

func (p *ProtocolCount) Dispatch() error {
	for {
		dbg.Lvl3("waiting for message in", p.Entity().Addresses)
		select {
		case pc := <-p.PrepareCountChan:
			dbg.Lvl3("Received from", pc.TreeNode.Entity.Addresses,
				pc.TreeNode.Id)
			p.FuncPC()
		case c := <-p.CountChan:
			p.FuncC(c)
		case <-time.After(time.Second * 10):
			dbg.LLvl3("Timeout while waiting for children")
			p.FuncC(nil)
		case _ = <-p.Quit:
			return nil
		}
	}
}

func (p *ProtocolCount) FuncPC() {
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			dbg.Lvl3("Sending to", c.Entity.Addresses, c.Id)
			p.SendTo(c, &PrepareCount{})
		}
	} else {
		p.FuncC(nil)
	}
}

func (p *ProtocolCount) FuncC(cc []CountMsg) {
	count := 1
	for _, c := range cc {
		count += c.Count.Children
	}
	if !p.IsRoot() {
		dbg.Lvl3("Sending to", p.Parent().Id, p.Parent().Entity.Addresses)
		p.SendTo(p.Parent(), &Count{count})
	} else {
		p.Count <- count
	}
	p.Quit <- true
	p.Done()
}

// Starts the protocol
func (p *ProtocolCount) Start() error {
	// Send an empty message
	dbg.Lvl3("Starting to count")
	p.FuncPC()
	return nil
}
