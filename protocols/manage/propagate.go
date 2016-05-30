package manage

import (
	"sync"

	"time"

	"reflect"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.ProtocolRegisterName("Propagate", NewPropagateProtocol)
}

// Propagate is a protocol that sends some data to all attached nodes
// and waits for confirmation before returning.
type Propagate struct {
	*sda.TreeNodeInstance
	onData    func(network.ProtocolMessage)
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

// CreateProtocolEntity is the necessary interface to start a protocol.
// It is implemented by Service and Overlay.
type CreateProtocolEntity interface {
	CreateProtocolService(t *sda.Tree, name string) (sda.ProtocolInstance, error)
	Entity() *network.Entity
}

// PropagateSendData is the message to pass the data to the children
type PropagateSendData struct {
	// Data is the data to transmit
	Data []byte
	// Msec is the timeout in milliseconds
	Msec int
}

// PropagateReply is sent from the children back to the root
type PropagateReply struct {
	Level int
}

// StartAndWait starts the propagation protocol and blocks till everything
// is OK or the timeout has been reached
func PropagateStartAndWait(ci CreateProtocolEntity, el *sda.EntityList, msg network.ProtocolMessage, msec int, f func(network.ProtocolMessage)) (int, error) {
	//dbg.Print(el, tree.Dump())
	tree := el.GenerateNaryTreeWithRoot(8, ci.Entity())
	dbg.Lvl2("Starting to propagate", reflect.TypeOf(msg))
	pi, err := ci.CreateProtocolService(tree, "Propagate")
	if err != nil {
		return -1, err
	}
	d, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return -1, err
	}
	protocol := pi.(*Propagate)
	protocol.Lock()
	protocol.sd.Data = d
	protocol.sd.Msec = msec
	protocol.onData = f

	done := make(chan int)
	protocol.onDoneCb = func(i int) { done <- i }
	protocol.Unlock()
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
	dbg.Lvl4("going to contact", p.Root().Entity)
	p.SendTo(p.Root(), p.sd)
	return nil
}

// Dispatch can handle timeouts
func (p *Propagate) Dispatch() error {
	process := true
	dbg.Lvl4(p.Entity())
	for process {
		p.Lock()
		timeout := time.Millisecond * time.Duration(p.sd.Msec)
		p.Unlock()
		select {
		case msg := <-p.ChannelSD:
			dbg.Lvl3(p.Entity(), "Got data from", msg.Entity)
			if p.onData != nil {
				_, netMsg, err := network.UnmarshalRegistered(msg.Data)
				if err == nil {
					p.onData(netMsg)
				}
			}
			if !p.IsRoot() {
				dbg.Lvl3(p.Entity(), "Sending to parent")
				p.SendToParent(&PropagateReply{})
			}
			if p.IsLeaf() {
				process = false
			} else {
				dbg.Lvl3(p.Entity(), "Sending to children")
				p.SendToChildren(&msg.PropagateSendData)
			}
		case <-p.ChannelReply:
			p.received++
			dbg.Lvl4(p.Entity(), "received:", p.received, p.subtree)
			if !p.IsRoot() {
				p.SendToParent(&PropagateReply{})
			}
			if p.received == p.subtree {
				process = false
			}
		case <-time.After(timeout):
			dbg.Fatal("Timeout")
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
func (p *Propagate) RegisterOnData(fn func(network.ProtocolMessage)) {
	p.onData = fn
}

// Config stores the basic configuration for that protocol.
func (p *Propagate) Config(d []byte, msec int) {
	p.sd.Data = d
	p.sd.Msec = msec
}
