package channels

import (
	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

func init() {
	network.RegisterPacketType(Announce{})
	network.RegisterPacketType(Reply{})
	sda.GlobalProtocolRegister("ExampleChannels", NewExampleChannels)
}

// ProtocolExampleChannels just holds a message that is passed to all children.
// It also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type ProtocolExampleChannels struct {
	*sda.TreeNodeInstance
	Message         string
	ChildCount      chan int
	ChannelAnnounce chan StructAnnounce
	ChannelReply    chan []StructReply
}

// NewExampleChannels initialises the structure for use in one round
func NewExampleChannels(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
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
	for _, c := range p.Children() {
		if err := p.SendTo(c, &Announce{"Example is here"}); err != nil {
			log.Error(p.Info(), "failed to send Announcment to",
				c.Name(), err)
		}
	}
	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *ProtocolExampleChannels) Dispatch() error {
	for {
		select {
		case announcement := <-p.ChannelAnnounce:
			if !p.IsLeaf() {
				// If we have children, send the same message to all of them
				for _, c := range p.Children() {
					err := p.SendTo(c, &announcement.Announce)
					if err != nil {
						log.Error(p.Info(),
							"failed to send to",
							c.Name(), err)
					}
				}
			} else {
				// If we're the leaf, start to reply
				err := p.SendTo(p.Parent(), &Reply{1})
				if err != nil {
					log.Error(p.Info(), "failed to send reply to",
						p.Parent().Name(), err)
				}
				return nil
			}
		case reply := <-p.ChannelReply:
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
	}
}
