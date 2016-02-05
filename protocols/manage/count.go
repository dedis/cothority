package manage

import (
	"fmt"
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
	Timeout          int
	NetworkDelay     int
	PrepareCountChan chan struct {
		*sda.TreeNode
		PrepareCount
	}
	CountChan chan []CountMsg
}

type PrepareCount struct {
	Timeout int
}

type Count struct {
	Children int
}
type CountMsg struct {
	*sda.TreeNode
	Count
}

func NewCount(n *sda.Node) (sda.ProtocolInstance, error) {
	p := &ProtocolCount{
		Node:    n,
		Quit:    make(chan bool),
		Timeout: 1024,
		// This also includes the time to make a connection, eventually
		// re-try if the connection failed
		NetworkDelay: 100,
	}
	p.Count = make(chan int, 1)
	p.RegisterChannel(&p.CountChan)
	p.RegisterChannel(&p.PrepareCountChan)
	return p, nil
}

func (p *ProtocolCount) Myself() string {
	return fmt.Sprint(p.Entity().Addresses, p.Node.TokenID())
}

func (p *ProtocolCount) Dispatch() error {
	for {
		dbg.Lvl3(p.Myself(), "waiting for message during", p.Timeout)
		select {
		case pc := <-p.PrepareCountChan:
			dbg.Lvl3(p.Myself(), "received from", pc.TreeNode.Entity.Addresses,
				pc.Timeout)
			p.Timeout = pc.Timeout
			p.FuncPC()
		case c := <-p.CountChan:
			p.FuncC(c)
		case <-time.After(time.Duration(p.Timeout) * time.Millisecond):
			dbg.Lvl3(p.Myself(), "timed out while waiting for", p.Timeout)
			p.FuncC(nil)
		case _ = <-p.Quit:
			return nil
		}
	}
}

func (p *ProtocolCount) FuncPC() {
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			// This value depends on the network-delay.
			newTO := p.Timeout - p.NetworkDelay*2
			if newTO < 100 {
				newTO = 100
			}
			dbg.Lvl3(p.Myself(), "sending to", c.Entity.Addresses, c.Id, newTO)
			p.SendTo(c, &PrepareCount{Timeout: newTO})
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
		dbg.Lvl3(p.Myself(), "Sends to", p.Parent().Id, p.Parent().Entity.Addresses)
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
