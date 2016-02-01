package example

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	network.RegisterMessageType(MessageAnnounce{})
	network.RegisterMessageType(MessageReply{})
	sda.ProtocolRegisterName("ExampleHandler", NewExampleHandler)
}

// ProtocolExampleHandler just holds a message that is passed to all children. It
// also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type ProtocolExampleHandler struct {
	*sda.Node
	Message    string
	ChildCount chan int
}

// NewExampleHandler initialises the structure for use in one round
func NewExampleHandler(n *sda.Node) (sda.ProtocolInstance, error) {
	example := &ProtocolExampleHandler{
		Node:       n,
		ChildCount: make(chan int),
	}
	err := example.RegisterHandler(example.HandleAnnounce)
	if err != nil {
		return nil, errors.New("couldn't register announcement-handler: " + err.Error())
	}
	err = example.RegisterHandler(example.HandleReply)
	if err != nil {
		return nil, errors.New("couldn't register reply-handler: " + err.Error())
	}
	return example, nil
}

// Starts the protocol
func (p *ProtocolExampleHandler) Start() error {
	dbg.Lvl3("Starting example")
	return p.HandleAnnounce(StructAnnounce{p.TreeNode(),
		MessageAnnounce{"cothority rulez!"}})
}

// Dispatch takes the message and decides what function to call
func (p *ProtocolExampleHandler) Dispatch(m []*sda.SDAData) error {
	return nil
}

// HandleAnnounce is the first message and is used to send an ID that
// is stored in all nodes.
func (p *ProtocolExampleHandler) HandleAnnounce(msg StructAnnounce) error {
	p.Message = msg.Message
	if !p.IsLeaf() {
		// If we have children, send the same message to all of them
		for _, c := range p.Children() {
			err := p.SendTo(c, &msg.MessageAnnounce)
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
func (p *ProtocolExampleHandler) HandleReply(reply []StructReply) error {
	children := 1
	for _, c := range reply {
		children += c.ChildrenCount
	}
	dbg.Lvl3(p.Entity().Addresses, "is done with total of", children)
	if !p.IsRoot() {
		dbg.Lvl3("Sending to parent")
		return p.SendTo(p.Parent(), &MessageReply{children})
	} else {
		dbg.Lvl3("Root-node is done - nbr of children found:", children)
		p.ChildCount <- children
	}
	return nil
}
