// Skipchain Protocol
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

// ProtocolSkipchain Genesis
type ProtocolSkipchain struct {
	*sda.Node
	SetupDone    chan bool
	SkipChain    map[string]*SkipBlock
	LastBlock    crypto.HashId
	CurrentBlock SkipBlock
}

// NewSkipchain initialises the structures and create the genesis block
func NewSkipchain(n *sda.Node) (sda.ProtocolInstance, error) {

	CurrentBlock := &SkipBlock{Index: 0} //TODO fill the fields
	Skipchain := &ProtocolSkipchain{
		Node:      n,
		SetupDone: make(chan bool),
		SkipChain: make(map[string]*SkipBlock),
		LastBlock: CurrentBlock.Hash(),
	}
	err := Skipchain.RegisterHandler(Skipchain.HandleGenesis)
	if err != nil {
		return nil, errors.New("couldn't register genesis-handler: " + err.Error())
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
	return p.HandleGenesis(StructGenesis{p.TreeNode(),
		MessageGenesis{}})
}

// HandleGenesis is used to sign the Genesis blocks it maybe renamed to HandleNewBlock
func (p *ProtocolSkipchain) HandleGenesis(msg StructGenesis) error {

	err := p.StartSignature(p.CurrentBlock)
	if err != nil {
		return err
	}
	return nil
}

//StartSignature create a new CoSi round and signs the hash of the block
func (p *ProtocolSkipchain) StartSignature(block SkipBlock) error {
	proto, err := p.CreateNewNodeName("CoSi")
	if err != nil {
		return err
	}
	pcosi := proto.ProtocolInstance().(*cosi.ProtocolCosi)
	//TODO verify the proposal and wait for a positive answer from the cosi challenge phase
	pcosi.SigningMessage(block.Hash())
	pcosi.RegisterDoneCallback(func(chal, resp abstract.Secret) {
		dbg.Lvl3("Cosi is Done")
		block.Signature = &libcosi.Signature{chal, resp}
		p.HandlePropagate(StructPropagate{MessagePropagate: MessagePropagate{Block: &block}})
		p.SetupDone <- true
	})
	proto.StartProtocol()
	return nil
}

// HandlePropagate sends the signed block to the nodes who add it in their SkipList
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
