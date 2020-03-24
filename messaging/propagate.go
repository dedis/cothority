package messaging

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

func init() {
	network.RegisterMessage(PropagateSendData{})
	network.RegisterMessage(PropagateReply{})
}

// How long to wait before timing out on waiting for the time-out.
const initialWait = 100000 * time.Millisecond

// Propagate is a protocol that sends some data to all attached nodes
// and waits for confirmation before returning.
type Propagate struct {
	*onet.TreeNodeInstance
	onData    PropagationStore
	onDoneCb  func(int)
	sd        *PropagateSendData
	ChannelSD chan struct {
		*onet.TreeNode
		PropagateSendData
	}
	ChannelReply chan struct {
		*onet.TreeNode
		PropagateReply
	}

	allowedFailures int
	sync.Mutex
	closing chan bool
}

// PropagateSendData is the message to pass the data to the children
type PropagateSendData struct {
	// Data is the data to transmit
	Data []byte
	// How long the root will wait for the children before
	// timing out.
	Timeout time.Duration
}

// PropagateReply is sent from the children back to the root
type PropagateReply struct {
	Level int
}

// PropagationFunc starts the propagation protocol and blocks until all children
// minus the exception stored the new value or the timeout has been reached.
// The return value is the number of nodes that acknowledged having
// stored the new value or an error if the protocol couldn't start.
type PropagationFunc func(el *onet.Roster, msg network.Message, timeout time.Duration) (int, error)

// PropagationStore is the function that will store the new data.
type PropagationStore func(network.Message) error

// propagationContext is used for testing.
type propagationContext interface {
	ProtocolRegister(name string, protocol onet.NewProtocol) (onet.ProtocolID, error)
	ServerIdentity() *network.ServerIdentity
	CreateProtocol(name string, t *onet.Tree) (onet.ProtocolInstance, error)
}

// NewPropagationFunc registers a new protocol name with the context c and will
// set f as handler for every new instance of that protocol.
// The protocol will fail if more than thresh nodes per subtree fail to respond.
// If thresh == -1, the threshold defaults to len(n.Roster().List-1)/3. Thus, for a roster of
// 5, t = int(4/3) = 1, e.g. 1 node out of the 5 can fail.
func NewPropagationFunc(c propagationContext, name string, f PropagationStore, thresh int) (PropagationFunc, error) {
	pid, err := c.ProtocolRegister(name, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		// Make a local copy in order to avoid a data race.
		t := thresh
		if t == -1 {
			t = (len(n.Roster().List) - 1) / 3
		}
		p := &Propagate{
			sd:               &PropagateSendData{[]byte{}, initialWait},
			TreeNodeInstance: n,
			onData:           f,
			allowedFailures:  t,
			closing:          make(chan bool),
		}
		for _, h := range []interface{}{&p.ChannelSD, &p.ChannelReply} {
			if err := p.RegisterChannel(h); err != nil {
				return nil, err
			}
		}
		return p, nil
	})
	log.Lvl3("Registering new propagation for", c.ServerIdentity(),
		name, pid)
	return func(el *onet.Roster, msg network.Message, to time.Duration) (int, error) {
		rooted := el.NewRosterWithRoot(c.ServerIdentity())
		if rooted == nil {
			return 0, errors.New("we're not in the roster")
		}
		tree := rooted.GenerateNaryTree(8)
		if tree == nil {
			return 0, errors.New("Didn't find root in tree")
		}
		log.Lvl3(el.List[0].Address, "Starting to propagate", reflect.TypeOf(msg))
		pi, err := c.CreateProtocol(name, tree)
		if err != nil {
			return 0, err
		}
		return propagateStartAndWait(pi, msg, to, f)
	}, err
}

// Separate function for testing
func propagateStartAndWait(pi onet.ProtocolInstance, msg network.Message, to time.Duration, f PropagationStore) (int, error) {
	d, err := network.Marshal(msg)
	if err != nil {
		return 0, err
	}
	protocol := pi.(*Propagate)
	protocol.Lock()
	protocol.sd.Data = d
	protocol.sd.Timeout = to
	protocol.onData = f

	done := make(chan int)
	protocol.onDoneCb = func(i int) { done <- i }
	protocol.Unlock()
	if err = protocol.Start(); err != nil {
		return 0, err
	}
	select {
	case replies := <-done:
		return replies, nil
	case <-protocol.closing:
		return 0, nil
	}
}

// Start will contact everyone and make the connections
func (p *Propagate) Start() error {
	log.Lvl4("going to contact", p.Root().ServerIdentity)
	p.SendTo(p.Root(), p.sd)
	return nil
}

// Dispatch can handle timeouts
func (p *Propagate) Dispatch() error {
	process := true
	var received int
	log.Lvl4(p.ServerIdentity(), "Start dispatch")
	defer p.Done()
	defer func() {
		if p.IsRoot() {
			if p.onDoneCb != nil {
				p.onDoneCb(received + 1)
			}
		}
	}()

	var gotSendData bool
	var errs []error
	subtreeCount := p.TreeNode().SubtreeCount()

	for process {
		p.Lock()
		timeout := p.sd.Timeout
		log.Lvl4("Got timeout", timeout, "from SendData")
		p.Unlock()
		select {
		case msg := <-p.ChannelSD:
			if gotSendData {
				log.Error("already got msg")
				continue
			}
			gotSendData = true
			log.Lvl3(p.ServerIdentity(), "Got data from", msg.ServerIdentity, "and setting timeout to", msg.Timeout)
			p.sd.Timeout = msg.Timeout
			if p.onData != nil {
				_, netMsg, err := network.Unmarshal(msg.Data, p.Suite())
				if err != nil {
					log.Lvlf2("Unmarshal failed with %v", err)
				} else {
					err := p.onData(netMsg)
					if err != nil {
						log.Lvlf2("Propagation callback failed: %v", err)
					}
				}
			}
			if !p.IsRoot() {
				log.Lvl3(p.ServerIdentity(), "Sending to parent")
				if err := p.SendToParent(&PropagateReply{}); err != nil {
					return err
				}
			}
			if p.IsLeaf() {
				process = false
			} else {
				log.Lvl3(p.ServerIdentity(), "Sending to children")
				if errs = p.SendToChildrenInParallel(&msg.PropagateSendData); len(errs) != 0 {
					var errsStr []string
					for _, e := range errs {
						errsStr = append(errsStr, e.Error())
					}
					if len(errs) > p.allowedFailures {
						return errors.New(strings.Join(errsStr, "\n"))
					}
					log.Lvl2("Error while sending to children:", errsStr)
				}
			}
		case <-p.ChannelReply:
			if !gotSendData {
				log.Error("got response before send")
				continue
			}
			received++
			log.Lvl4(p.ServerIdentity(), "received:", received, subtreeCount)
			if !p.IsRoot() {
				if err := p.SendToParent(&PropagateReply{}); err != nil {
					return err
				}
			}
			// Only wait for the number of children that successfully received our message.
			if received == subtreeCount-len(errs) && received >= subtreeCount-p.allowedFailures {
				process = false
			}
		case <-time.After(timeout):
			if received < subtreeCount-p.allowedFailures {
				_, _, err := network.Unmarshal(p.sd.Data, p.Suite())
				return fmt.Errorf("Timeout of %s reached, got %v but need %v, err: %v",
					timeout, received, subtreeCount-p.allowedFailures, err)
			}
			process = false
		case <-p.closing:
			process = false
			p.onDoneCb = nil
		}
	}
	log.Lvl3(p.ServerIdentity(), "done, isroot:", p.IsRoot())
	return nil
}

// RegisterOnDone takes a function that will be called once the data has been
// sent to the whole tree. It receives the number of nodes that replied
// successfully to the propagation.
func (p *Propagate) RegisterOnDone(fn func(int)) {
	p.onDoneCb = fn
}

// RegisterOnData takes a function that will be called for that node if it
// needs to update its data.
func (p *Propagate) RegisterOnData(fn PropagationStore) {
	p.onData = fn
}

// Config stores the basic configuration for that protocol.
func (p *Propagate) Config(d []byte, timeout time.Duration) {
	p.sd.Data = d
	p.sd.Timeout = timeout
}

// Shutdown informs the Dispatch method to stop
// waiting.
func (p *Propagate) Shutdown() error {
	close(p.closing)
	return nil
}
