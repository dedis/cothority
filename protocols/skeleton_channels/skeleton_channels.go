package skeleton_channels

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	network.RegisterMessageType(MessageAnnounce{})
	network.RegisterMessageType(MessageReply{})
	sda.ProtocolRegisterName("SkeletonChannels", NewSkeletonChannels)
}

// ProtocolSkeletonChannels just holds a message that is passed to all children. It
// also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type ProtocolSkeletonChannels struct {
	*sda.Node
	Message         string
	ChildCount      chan int
	ChannelAnnounce chan StructAnnounce
	ChannelReply    chan []StructReply
}

// NewSkeletonChannels initialises the structure for use in one round
func NewSkeletonChannels(n *sda.Node) (sda.ProtocolInstance, error) {
	SkeletonChannels := &ProtocolSkeletonChannels{
		Node:       n,
		ChildCount: make(chan int),
	}
	err := SkeletonChannels.RegisterChannel(&SkeletonChannels.ChannelAnnounce)
	if err != nil {
		return nil, errors.New("couldn't register announcement-channel: " + err.Error())
	}
	err = SkeletonChannels.RegisterChannel(&SkeletonChannels.ChannelReply)
	if err != nil {
		return nil, errors.New("couldn't register reply-channel: " + err.Error())
	}
	go SkeletonChannels.DispatchChannels()
	return SkeletonChannels, nil
}

// Starts the protocol
func (p *ProtocolSkeletonChannels) Start() error {
	dbg.Lvl3("Starting SkeletonChannels")
	for _, c := range p.Children() {
		p.SendTo(c, &MessageAnnounce{"Skeleton is here"})
	}
	return nil
}

// Dispatch takes the message and decides what function to call
func (p *ProtocolSkeletonChannels) Dispatch(m []*sda.SDAData) error {
	return nil
}

// DispatchChannels is an infinite loop to handle messages from channels
func (p *ProtocolSkeletonChannels) DispatchChannels() {
	for {
		select {
		case announcement := <-p.ChannelAnnounce:
			if !p.IsLeaf() {
				// If we have children, send the same message to all of them
				for _, c := range p.Children() {
					p.SendTo(c, &announcement.MessageAnnounce)
				}
			} else {
				// If we're the leaf, start to reply
				p.SendTo(p.Parent(), &MessageReply{1})
			}
		case reply := <-p.ChannelReply:
			children := 1
			for _, c := range reply {
				children += c.ChildrenCount
			}
			dbg.Lvl3(p.Entity().Addresses, "is done with total of", children)
			if !p.IsRoot() {
				dbg.Lvl3("Sending to parent")
				p.SendTo(p.Parent(), &MessageReply{children})
			} else {
				dbg.Lvl3("Root-node is done - nbr of children found:", children)
				p.ChildCount <- children
			}
		}
	}
}
