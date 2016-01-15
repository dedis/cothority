/*
Implements a full test with
- a test-protocol (two steps)
- 4 local nodes
- tree-graph for the nodes
- passing of messages
*/
package example

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

var Done chan bool

func init() {
	sda.ProtocolRegisterName("Example", NewExample)
}

// ProtocolExample just holds a message that is passed to all children.
type ProtocolExample struct {
	*sda.ProtocolStruct
	Message string
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
func NewExample(h *sda.Host, t *sda.TreeNode, tok *sda.Token) sda.ProtocolInstance {
	if Done == nil {
		Done = make(chan bool, 1)
	}
	return &ProtocolExample{
		ProtocolStruct: sda.NewProtocolStruct(h, t, tok),
	}
}

// Starts the protocol
func (p *ProtocolExample) Start() error {
	dbg.Lvl3("Starting example")
	return p.Send(p.Children[0], &MessageAnnounce{"cothority rulez!"})
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
		for _, c := range p.Children {
			err := p.Send(c, msg)
			if err != nil {
				return err
			}
		}
	} else {
		// If we're the leaf, start to reply
		return p.Send(p.Parent, &MessageReply{1})
	}
	return nil
}

// HandleReply is the message going up the tree and holding a counter
// to verify the number of nodes.
func (p *ProtocolExample) HandleReply(m *sda.SDAData) error {
	msg := m.Msg.(MessageReply)
	msg.Children += len(p.Children)
	Done <- true
	dbg.Lvl3("We're done")
	if p.Parent != nil {
		return p.Send(p.Parent, msg)
	}
	return nil
}
