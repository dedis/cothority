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
	sda.ProtocolRegister("Example", NewProtocolExample)
}

/*
ProtocolExample just holds a message that is passed to all children.
*/
type ProtocolExample struct {
	sda.Node
	sda.TreePeer
	Message string
}

/*
MessageAnnounce is used to pass a message to all children
*/
type MessageAnnounce struct {
	Message string
}

/*
MessageReply returns the count of all children
*/
type MessageReply struct {
	Children int
}

/*
NewProtocolExample initialises the structure for use in one round
*/
func (p *ProtocolExample) NewProtocolExample(n *sda.Node, t *sda.TreePeer) {
	p.Node = n
	p.TreePeer = t
}

/*
Dispatch takes the message and decides what function to call
*/
func (p *ProtocolExample) Dispatch(m []*sda.Message) error {
	switch m[0].MessageType {
	case 0:
		return p.HandleAnnounce(m[0])
	case 1:
		return p.HandleReply(m)
	}
	return sda.NoSuchState
}

/*
MessageAnnounce is the first message and is used to send an ID that
is stored in all nodes.
*/
func (p *ProtocolExample) HandleAnnounce(m *sda.Message) error {
	msg := m.Message.(MessageAnnounce)
	p.Message = msg.Message
	if !p.IsLeaf() {
		// If we have children, send the same message to all of them
		for nil, c := range p.Children() {
			err := p.SendMessage(c, msg)
			if err != nil {
				return err
			}
		}
	} else {
		// If we're the leaf, start to reply
		return p.SendMessage(p.Parent(), &MessageReply{1})
	}
	return nil
}

/*
MessageReply is the message going up the tree and holding a counter
to verify the number of nodes.
*/
func (p *ProtocolExample) HandleReply(m *sda.Message) error {
	msg := m.Message.(MessageReply)
	msg.Children += len(p.Children())
	return p.SendMessage(p.Parent(), msg)
}
