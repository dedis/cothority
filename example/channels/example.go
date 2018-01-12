package channels

import (
	"errors"

	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	network.RegisterMessage(Announce{})
	network.RegisterMessage(Reply{})
	onet.GlobalProtocolRegister("ExampleChannels", NewExampleChannels)
}

// ProtocolExampleChannels just holds a message that is passed to all children.
// It also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type ProtocolExampleChannels struct {
	*onet.TreeNodeInstance
	Message         string
	ChildCount      chan int
	ChannelAnnounce chan StructAnnounce
	ChannelReply    chan []StructReply
}

// NewExampleChannels initialises the structure for use in one round
func NewExampleChannels(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	ExampleChannels := &ProtocolExampleChannels{
		TreeNodeInstance: n,
		ChildCount:       make(chan int),
	}
	err := ExampleChannels.RegisterChannel(&ExampleChannels.ChannelAnnounce)
	if err != nil {
		return nil, errors.New("couldn't register announcement-channel: " + err.Error())
	}
	err = ExampleChannels.RegisterChannel(&ExampleChannels.ChannelReply)
	if err != nil {
		return nil, errors.New("couldn't register reply-channel: " + err.Error())
	}
	return ExampleChannels, nil
}

// Start sends the Announce message to all children
func (p *ProtocolExampleChannels) Start() error {
	log.Lvl3("Starting ExampleChannels")
	p.ChannelAnnounce <- StructAnnounce{nil, Announce{"Example is here"}}
	return nil
}

// Dispatch imposes an order on the incoming messages by waiting only on the
// appropriate channels
func (p *ProtocolExampleChannels) Dispatch() error {
	announcement := <-p.ChannelAnnounce
	if p.IsLeaf() {
		// If we're the leaf, start to reply
		err := p.SendTo(p.Parent(), &Reply{1})
		if err != nil {
			log.Error(p.Info(), "failed to send reply to",
				p.Parent().Name(), err)
		}
		// Leaf-node is done and doesn't wait on a reply
		return nil
	}

	// Send the same message to all our children
	for _, c := range p.Children() {
		err := p.SendTo(c, &announcement.Announce)
		if err != nil {
			log.Error(p.Info(),
				"failed to send to",
				c.Name(), err)
		}
	}

	// Wait for a reply of all our children
	reply := <-p.ChannelReply
	children := 1
	for _, c := range reply {
		children += c.ChildrenCount
	}
	log.Lvl3(p.ServerIdentity().Address, "is done with total of", children)
	if !p.IsRoot() {
		log.Lvl3("Sending to parent")
		err := p.SendTo(p.Parent(), &Reply{children})
		if err != nil {
			log.Error(p.Info(), "failed to reply to",
				p.Parent().Name(), err)
		}
	} else {
		log.Lvl3("Root-node is done - nbr of children found:", children)
		p.ChildCount <- children
	}
	return nil
}
