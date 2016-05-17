package manage

import (
	"sync"

	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.ProtocolRegisterName("Propagate", NewPropagateProtocol)
}

// Propagate is a protocol that sends some data to all attached nodes
// and waits for confirmation before returning.
type Propagate struct {
	*sda.TreeNodeInstance
	onData    func([]byte)
	onDoneCb  func(int)
	sd        *PropagateSendData
	ChannelSD chan struct {
		*sda.TreeNode
		PropagateSendData
	}
	ChannelReply chan struct {
		*sda.TreeNode
		PropagateReply
	}

	received int
	subtree  int
	sync.Mutex
}

type PropagateData interface {
	StoreData(data []byte) error
}

type CI interface {
	CreateProtocol(t *sda.Tree, name string) (sda.ProtocolInstance, error)
}

// SendData is the message to pass the data to the children
type PropagateSendData struct {
	// Data is the data to transmit
	Data []byte
	// Msec is the timeout in milliseconds
	Msec int
}

// Reply is sent from the children back to the root
type PropagateReply struct {
	Level int
}

// StartAndWait starts the propagation protocol and blocks till everything
// is OK or the timeout has been reached
func PropagateStartAndWait(ci CI, tree *sda.Tree, d []byte, msec int, f func([]byte)) (int, error) {
	dbg.Lvl2("Starting to propagate")
	pi, err := ci.CreateProtocol(tree, "Propagate")
	if err != nil {
		return -1, err
	}
	protocol := pi.(*Propagate)
	protocol.sd.Data = d
	protocol.sd.Msec = msec
	protocol.onData = f

	done := make(chan int)
	protocol.onDoneCb = func(i int) { done <- i }
	if err = protocol.Start(); err != nil {
		return -1, err
	}
	ret := <-done
	dbg.Lvl2("Finished propagation with", ret, "replies")
	return ret, nil
}

// NewPropagateProtocol returns a new Propagate protocol
func NewPropagateProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	p := &Propagate{
		sd:               &PropagateSendData{[]byte{}, 1000},
		TreeNodeInstance: n,
		received:         0,
		subtree:          n.TreeNode().SubtreeCount(),
	}
	for _, h := range []interface{}{&p.ChannelSD, &p.ChannelReply} {
		if err := p.RegisterChannel(h); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Start will contact everyone and make the connections
func (p *Propagate) Start() error {
	p.SendTo(p.Root(), p.sd)
	return nil
}

// Dispatch can handle timeouts
func (p *Propagate) Dispatch() error {
	process := true
	for process {
		select {
		case msg := <-p.ChannelSD:
			if p.onData != nil {
				p.onData(msg.Data)
			}
			if !p.IsLeaf() {
				p.SendToChildren(&msg.PropagateSendData)
			} else {
				p.SendToParent(&PropagateReply{})
				process = false
			}
		case <-p.ChannelReply:
			p.received++
			if !p.IsRoot() {
				p.SendToParent(&PropagateReply{})
			}
			if p.received == p.subtree {
				p.SendToParent(&PropagateReply{})
				process = false
			}
		case <-time.After(time.Millisecond * time.Duration(p.sd.Msec)):
			dbg.Lvl3(p.Entity(), "Timeout")
			process = false
		}
	}
	if p.IsRoot() {
		if p.onDoneCb != nil {
			p.onDoneCb(p.received + 1)
		}
	}
	p.Done()
	return nil
}

// RegisterOnDone takes a function that will be called once all connections
// are set up. The argument to the function is the number of children that
// sent OK after the propagation
func (p *Propagate) RegisterOnDone(fn func(int)) {
	p.onDoneCb = fn
}

// RegisterOnData takes a function that will be called once all connections
// are set up. The argument to the function is the number of children that
// sent OK after the propagation
func (p *Propagate) RegisterOnData(fn func([]byte)) {
	p.onData = fn
}

// Config stores the basic configuration for that protocol.
func (p *Propagate) Config(d []byte, msec int) {
	p.sd.Data = d
	p.sd.Msec = msec
}
