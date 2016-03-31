// Holds a message that is passed to all children, using handlers.
package skipchain

import (
	"errors"
	libcosi "github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/cosi"
	"github.com/dedis/crypto/abstract"
)

func init() {
	sda.ProtocolRegisterName("Skipchain", NewSkipchain)
}

// ProtocolSkipchain creates
type ProtocolSkipchain struct {
	*sda.Node
	SetupDone chan bool
	SkipChain map[string]*SkipBlock
	LastBlock crypto.HashId
}

// NewSkipchain initialises the structure for use in one round
func NewSkipchain(n *sda.Node) (sda.ProtocolInstance, error) {
	block := &SkipBlock{Index: 0} //TODO fill the fields
	Skipchain := &ProtocolSkipchain{
		Node:      n,
		SetupDone: make(chan bool),
		SkipChain: make(map[string]*SkipBlock),
		LastBlock: block.Hash(),
	}
	Skipchain.SkipChain[string(Skipchain.LastBlock)] = block
	err := Skipchain.RegisterHandler(Skipchain.HandleCreate)
	if err != nil {
		return nil, errors.New("couldn't register announcement-handler: " + err.Error())
	}
	err = Skipchain.RegisterHandler(Skipchain.HandleReply)
	if err != nil {
		return nil, errors.New("couldn't register reply-handler: " + err.Error())
	}
	err = Skipchain.RegisterHandler(Skipchain.HandlePropagate)
	if err != nil {
		return nil, errors.New("couldn't register propagete-handler: " + err.Error())
	}
	return Skipchain, nil
}

// Starts the protocol
func (p *ProtocolSkipchain) Start() error {
	dbg.Lvl3("Starting Skipchain")
	return p.HandleCreate(StructCreate{p.TreeNode(),
		MessageCreate{}})
}

// HandleCreate is used to check that the tree is setup
func (p *ProtocolSkipchain) HandleCreate(msg StructCreate) error {

	if !p.IsLeaf() {
		// If we have children, send the same message to all of them
		for _, c := range p.Children() {
			err := p.SendTo(c, &msg.MessageCreate)
			if err != nil {
				return err
			}
		}
	} else {
		// If we're the leaf, start to reply
		return p.SendTo(p.Parent(), &MessageReply{})
	}
	return nil
}

// HandleReply is the message going up the tree and holding a counter
// to verify the number of nodes.
func (p *ProtocolSkipchain) HandleReply(reply []StructReply) error {

	dbg.Lvl3(p.Entity().Addresses, "is done with total of")
	if !p.IsRoot() {
		dbg.Lvl3("Sending to parent")
		return p.SendTo(p.Parent(), &MessageReply{})
	} else {
		dbg.Lvl3("Root-node is done - nbr of children found:")
		err := p.StartSignature()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *ProtocolSkipchain) StartSignature() error {
	proto, err := p.CreateNewNodeName("CoSi")
	if err != nil {
		return err
	}
	block := &SkipBlock{Index: p.SkipChain[string(p.LastBlock)].Index + 1} //TODO intiliaze rest
	pcosi := proto.ProtocolInstance().(*cosi.ProtocolCosi)
	//TODO verify the proposal and wait for a positive answer from the cosi challenge phase
	pcosi.SigningMessage(block.Hash())
	pcosi.RegisterDoneCallback(func(chal, resp abstract.Secret) {
		dbg.Lvl3("Cosi is Done")
		block.Signature = &libcosi.Signature{chal, resp}

		p.HandlePropagate(StructPropagate{MessagePropagate: MessagePropagate{Block: block}})
		p.SetupDone <- true
	})
	proto.StartProtocol()
	return nil
}

// to verify the number of nodes.
func (p *ProtocolSkipchain) HandlePropagate(prop StructPropagate) error {
	p.LastBlock = prop.Block.Hash()
	p.SkipChain[string(p.LastBlock)] = prop.Block
	if !p.IsLeaf() {
		// If we have children, send the same message to all of them
		for _, c := range p.Children() {
			err := p.SendTo(c, &prop.MessagePropagate)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
