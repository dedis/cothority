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
	network.RegisterMessageType(MsgPrepareCount{})
	network.RegisterMessageType(MsgCount{})
	sda.ProtocolRegisterName("Count", NewCount)
}

type ProtocolCount struct {
	*sda.Node
	Count           chan int
	Quit            chan bool
	MsgPrepareCount chan struct {
		*sda.TreeNode
		MsgPrepareCount
	}
	MsgCount chan []struct {
		*sda.TreeNode
		MsgCount
	}
}

type MsgPrepareCount struct{}

type MsgCount struct {
	Children int
}

func NewCount(n *sda.Node) (sda.ProtocolInstance, error) {
	p := &ProtocolCount{
		Node: n,
		Quit: make(chan bool),
	}
	p.Count = make(chan int, 1)
	p.RegisterChannel(&p.MsgPrepareCount)
	p.RegisterChannel(&p.MsgCount)
	go p.DispatchChannels()
	return p, nil
}

func (p *ProtocolCount) DispatchChannels() {
	for {
		dbg.Lvl3("waiting for message in", p.Entity().Addresses)
		select {
		case _ = <-p.MsgPrepareCount:
			p.FuncPC()
		case c := <-p.MsgCount:
			p.FuncC(c)
		case <-time.After(time.Second * 10):
			dbg.LLvl3("Timeout while waiting for children")
			p.FuncC(nil)
		case _ = <-p.Quit:
			return
		}
	}
}

func (p *ProtocolCount) FuncPC() {
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			dbg.Lvl3("Sending to", c.Entity.Addresses)
			p.SendTo(c, &MsgPrepareCount{})
		}
	} else {
		p.FuncC(nil)
	}
}

func (p *ProtocolCount) FuncC(c []struct {
	*sda.TreeNode
	MsgCount
}) {
	count := 1
	for _, c := range c {
		count += c.MsgCount.Children
	}
	if !p.IsRoot() {
		dbg.Lvl3("Sending to", p.Parent().Entity.Addresses)
		p.SendTo(p.Parent(), &MsgCount{count})
	} else {
		p.Count <- count
	}
	p.Quit <- true
	p.Done()
}

// Starts the protocol
func (p *ProtocolCount) Start() error {
	// Send an empty message
	dbg.LLvl3("Starting to count")
	p.FuncPC()
	return nil
}

// Dispatch takes the message and decides what function to call
func (p *ProtocolCount) Dispatch(m []*sda.SDAData) error {
	return nil
}
