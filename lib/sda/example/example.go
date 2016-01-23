package example

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.ProtocolRegisterName("Example", NewExample)
}

// ProtocolExample just holds a message that is passed to all children. It
// also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type ProtocolExample struct {
	*sda.Node
	Message    string
	ChildCount chan int
}

// MessageAnnounce is used to pass a message to all children
type MessageAnnounce struct {
	Message string
}

var MessageAnnounceType = network.RegisterMessageType(MessageAnnounce{})

// MessageReply returns the count of all children
type MessageReply struct {
	Children int
}

var MessageReplyType = network.RegisterMessageType(MessageReply{})

// NewProtocolInstance initialises the structure for use in one round
func NewExample(n *sda.Node) sda.ProtocolInstance {
	return &ProtocolExample{
		Node:       n,
		ChildCount: make(chan int),
	}
}

// Starts the protocol
func (p *ProtocolExample) Start() error {
	dbg.Lvl3("Starting example")
	return p.SendTo(p.Children()[0], &MessageAnnounce{"cothority rulez!"})
}

// Dispatch takes the message and decides what function to call
func (p *ProtocolExample) Dispatch(m []*sda.SDAData) error {
	dbg.Lvl3("Got a message:", m[0])
	switch m[0].MsgType {
	case MessageAnnounceType:
		return p.HandleAnnounce(m[0])
	case MessageReplyType:
		return p.HandleReply(m[0])
	}
	return sda.NoSuchState
}

// HandleAnnounce is the first message and is used to send an ID that
// is stored in all nodes.
func (p *ProtocolExample) HandleAnnounce(m *sda.SDAData) error {
	msg := m.Msg.(MessageAnnounce)
	p.Message = msg.Message
	if !p.IsLeaf() {
		// If we have children, send the same message to all of them
		for _, c := range p.Children() {
			err := p.SendTo(c, msg)
			if err != nil {
				return err
			}
		}
	} else {
		// If we're the leaf, start to reply
		return p.SendTo(p.Parent(), &MessageReply{1})
	}
	return nil
}

// HandleReply is the message going up the tree and holding a counter
// to verify the number of nodes.
func (p *ProtocolExample) HandleReply(m *sda.SDAData) error {
	msg := m.Msg.(MessageReply)
	msg.Children += len(p.Children())
	dbg.Lvl3("We're done")
	if p.Parent() != nil {
		return p.SendTo(p.Parent(), msg)
	} else {
		p.ChildCount <- msg.Children
	}
	return nil
}
