package example_channels

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	network.RegisterMessageType(Announce{})
	network.RegisterMessageType(Reply{})
	sda.ProtocolRegisterName("ExampleChannels", NewExampleChannels)
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

// Starts the protocol
func (p *ProtocolExampleChannels) Start() error {
	dbg.Lvl3("Starting ExampleChannels")
	for _, c := range p.Children() {
		if err := p.SendTo(c, &Announce{"Example is here"}); err != nil {
			dbg.Error(p.Info(), "failed to send Announcment to",
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
						dbg.Error(p.Info(),
							"failed to send to",
							c.Name(), err)
					}
				}
			} else {
				// If we're the leaf, start to reply
				err := p.SendTo(p.Parent(), &Reply{1})
				if err != nil {
					dbg.Error(p.Info(), "failed to send reply to",
						p.Parent().Name(), err)
				}
				return nil
			}
		case reply := <-p.ChannelReply:
			children := 1
			for _, c := range reply {
				children += c.ChildrenCount
			}
			dbg.Lvl3(p.Entity().Addresses, "is done with total of", children)
			if !p.IsRoot() {
				dbg.Lvl3("Sending to parent")
				err := p.SendTo(p.Parent(), &Reply{children})
				if err != nil {
					dbg.Error(p.Info(), "failed to reply to",
						p.Parent().Name(), err)
				}
			} else {
				dbg.Lvl3("Root-node is done - nbr of children found:", children)
				p.ChildCount <- children
			}
			return nil
		}
	}
}
