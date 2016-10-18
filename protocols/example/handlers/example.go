package handlers

import (
	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

func init() {
	network.RegisterPacketType(Announce{})
	network.RegisterPacketType(Reply{})
	sda.GlobalProtocolRegister("ExampleHandlers", NewExampleHandlers)
}

// ProtocolExampleHandlers just holds a message that is passed to all children. It
// also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type ProtocolExampleHandlers struct {
	*sda.TreeNodeInstance
	Message    string
	ChildCount chan int
}

// NewExampleHandlers initialises the structure for use in one round
func NewExampleHandlers(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	ExampleHandlers := &ProtocolExampleHandlers{
		TreeNodeInstance: n,
		ChildCount:       make(chan int),
	}
	err := ExampleHandlers.RegisterHandler(ExampleHandlers.HandleAnnounce)
	if err != nil {
		return nil, errors.New("couldn't register announcement-handler: " + err.Error())
	}
	err = ExampleHandlers.RegisterHandler(ExampleHandlers.HandleReply)
	if err != nil {
		return nil, errors.New("couldn't register reply-handler: " + err.Error())
	}
	return ExampleHandlers, nil
}

// Start sends the Announcement-message to all children
func (p *ProtocolExampleHandlers) Start() error {
	log.Lvl3("Starting ExampleHandlers")
	return p.HandleAnnounce(StructAnnounce{p.TreeNode(),
		Announce{"cothority rulez!"}})
}

// HandleAnnounce is the first message and is used to send an ID that
// is stored in all nodes.
func (p *ProtocolExampleHandlers) HandleAnnounce(msg StructAnnounce) error {
	p.Message = msg.Message
	if !p.IsLeaf() {
		// If we have children, send the same message to all of them
		for _, c := range p.Children() {
			err := p.SendTo(c, &msg.Announce)
			if err != nil {
				return err
			}
		}
	} else {
		// If we're the leaf, start to reply
		return p.SendTo(p.Parent(), &Reply{1})
	}
	return nil
}

// HandleReply is the message going up the tree and holding a counter
// to verify the number of nodes.
func (p *ProtocolExampleHandlers) HandleReply(reply []StructReply) error {
	children := 1
	for _, c := range reply {
		children += c.ChildrenCount
	}
	log.Lvl3(p.ServerIdentity().Address, "is done with total of", children)
	if !p.IsRoot() {
		log.Lvl3("Sending to parent")
		return p.SendTo(p.Parent(), &Reply{children})
	}
	log.Lvl3("Root-node is done - nbr of children found:", children)
	p.ChildCount <- children
	return nil
}
