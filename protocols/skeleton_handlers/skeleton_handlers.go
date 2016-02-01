package skeleton_handlers

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	network.RegisterMessageType(MessageAnnounce{})
	network.RegisterMessageType(MessageReply{})
	sda.ProtocolRegisterName("SkeletonHandlers", NewSkeletonHandlers)
}

// ProtocolSkeletonHandlers just holds a message that is passed to all children. It
// also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type ProtocolSkeletonHandlers struct {
	*sda.Node
	Message    string
	ChildCount chan int
}

// NewSkeletonHandlers initialises the structure for use in one round
func NewSkeletonHandlers(n *sda.Node) (sda.ProtocolInstance, error) {
	SkeletonHandlers := &ProtocolSkeletonHandlers{
		Node:       n,
		ChildCount: make(chan int),
	}
	err := SkeletonHandlers.RegisterHandler(SkeletonHandlers.HandleAnnounce)
	if err != nil {
		return nil, errors.New("couldn't register announcement-handler: " + err.Error())
	}
	err = SkeletonHandlers.RegisterHandler(SkeletonHandlers.HandleReply)
	if err != nil {
		return nil, errors.New("couldn't register reply-handler: " + err.Error())
	}
	return SkeletonHandlers, nil
}

// Starts the protocol
func (p *ProtocolSkeletonHandlers) Start() error {
	dbg.Lvl3("Starting SkeletonHandlers")
	return p.HandleAnnounce(StructAnnounce{p.TreeNode(),
		MessageAnnounce{"cothority rulez!"}})
}

// Dispatch takes the message and decides what function to call
func (p *ProtocolSkeletonHandlers) Dispatch(m []*sda.SDAData) error {
	return nil
}

// HandleAnnounce is the first message and is used to send an ID that
// is stored in all nodes.
func (p *ProtocolSkeletonHandlers) HandleAnnounce(msg StructAnnounce) error {
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
func (p *ProtocolSkeletonHandlers) HandleReply(reply []StructReply) error {
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
