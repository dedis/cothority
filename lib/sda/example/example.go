/*
Implements a full test with
- a test-protocol (two steps)
- 4 local nodes
- tree-graph for the nodes
- passing of messages
*/
package example

import "github.com/dedis/cothority/lib/sda"

func init() {
	sda.ProtocolRegisterName("Example", NewProtocolInstance)
}

// ProtocolExample just holds a message that is passed to all children.
type ProtocolExample struct {
	sda.ProtocolStruct
	Message string
}

// MessageAnnounce is used to pass a message to all children
type MessageAnnounce struct {
	Message string
}

// MessageReply returns the count of all children
type MessageReply struct {
	Children int
}

// NewProtocolInstance initialises the structure for use in one round
func NewProtocolInstance(h *sda.Host, t *sda.TreeNode, tok *sda.Token) *ProtocolExample {
	return &ProtocolExample{
		ProtocolStruct: sda.NewProtocolStruct(h, t, tok),
	}
}

// Dispatch takes the message and decides what function to call
func (p *ProtocolExample) Dispatch(m []*sda.SDAData) error {
	switch m[0].MsgType {
	case 0:
		return p.HandleAnnounce(m[0])
	case 1:
		return p.HandleReply(m)
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
		for nil, c := range p.Children() {
			err := p.Send(c, msg)
			if err != nil {
				return err
			}
		}
	} else {
		// If we're the leaf, start to reply
		return p.Send(p.Parent(), &MessageReply{1})
	}
	return nil
}

// HandleReply is the message going up the tree and holding a counter
// to verify the number of nodes.
func (p *ProtocolExample) HandleReply(m *sda.SDAData) error {
	msg := m.Msg.(MessageReply)
	msg.Children += len(p.Children())
	return p.Send(p.Parent(), msg)
}
