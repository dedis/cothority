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
	sda.ProtocolRegisterName("ExampleChannel", NewExampleChannel)
}

// ProtocolExampleChannel just holds a message that is passed to all children. It
// also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type ProtocolExampleChannel struct {
	*sda.Node
	Message      string
	ChildCount   chan int
	Announcement chan struct {
		sda.TreeNode
		MessageAnnounce
	}
	Reply chan []struct {
		sda.TreeNode
		MessageReply
	}
}

// NewExampleChannel initialises the structure for use in one round
func NewExampleChannel(n *sda.Node) (sda.ProtocolInstance, error) {
	example := &ProtocolExampleChannel{
		Node:       n,
		ChildCount: make(chan int),
	}
	err := example.RegisterChannel(&example.Announcement)
	if err != nil {
		return nil, errors.New("couldn't register announcement-channel")
	}
	err = example.RegisterChannel(&example.Reply)
	if err != nil {
		return nil, errors.New("couldn't register reply-channel")
	}
	go example.DispatchChannels()
	return example, nil
}

// Starts the protocol
func (p *ProtocolExampleChannel) Start() error {
	dbg.Lvl3("Starting example")
	return p.HandleAnnounce(MessageAnnounce{"cothority rulez!"})
}

// Dispatch takes the message and decides what function to call
func (p *ProtocolExampleChannel) Dispatch(m []*sda.SDAData) error {
	return nil
}

func (p *ProtocolExampleChannel) DispatchChannels() {
	for {
		dbg.Lvl3("waiting for message in", p.Entity().Addresses)
		select {
		case announce := <-p.Announcement:
			dbg.Lvl3("Got announcement", announce)
			err := p.HandleAnnounce(announce.MessageAnnounce)
			if err != nil {
				dbg.Error("Error in announcement:", err)
			}
		case reply := <-p.Reply:
			p.HandleReply(reply)
		}
	}
}

// HandleAnnounce is the first message and is used to send an ID that
// is stored in all nodes.
func (p *ProtocolExampleChannel) HandleAnnounce(msg MessageAnnounce) error {
	p.Message = msg.Message
	if !p.IsLeaf() {
		// If we have children, send the same message to all of them
		for _, c := range p.Children() {
			err := p.SendTo(c, &msg)
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
func (p *ProtocolExampleChannel) HandleReply(reply []struct {
	sda.TreeNode
	MessageReply
}) error {
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
