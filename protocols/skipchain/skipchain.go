// Skipchain Protocol
package skipchain

import (
	"errors"
	libcosi "github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
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
	LastBlock    []byte
	CurrentBlock SkipBlock
}

// NewSkipchain initialises the structures and create the genesis block
func NewSkipchain(n *sda.Node) (sda.ProtocolInstance, error) {

	Skipchain := &ProtocolSkipchain{
		Node:      n,
		SetupDone: make(chan bool),
		SkipChain: make(map[string]*SkipBlock),
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
	block := &SkipBlock{Index: 0, X_0: p.TreeNode().PublicAggregateSubTree, Nodes: p.Tree().List()}
	p.LastBlock = block.Hash()
	return p.HandleGenesis(StructGenesis{p.TreeNode(),
		MessageGenesis{Block: block}})
}

// HandleGenesis is used to sign the Genesis blocks
func (p *ProtocolSkipchain) HandleGenesis(msg StructGenesis) error {
	err := p.StartSignature(*msg.Block)
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
	//TODO verify the proposal and wait for a positive answer from the cosi challenge phase??? register cosi functions for that???
	pcosi.SigningMessage(block.Hash())
	pcosi.RegisterDoneCallback(func(chal, resp abstract.Secret) {
		dbg.Lvl3("Cosi is Done")
		block.Signature = &libcosi.Signature{chal, resp}
		block.Nodes = nil
		p.HandlePropagate(StructPropagate{MessagePropagate: MessagePropagate{Block: &block}})
		p.SetupDone <- true
	})
	proto.StartProtocol()
	return nil
}

// HandlePropagate sends the signed block to the nodes who add it in their SkipList
func (p *ProtocolSkipchain) HandlePropagate(prop StructPropagate) error {
	//TODO if the block is propagated before now propagate only the signature on recieve nodes set block.Nodes to nil
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

// HandleNewBlock is used to sign new blocks, it is called by the application when something changes
func (p *ProtocolSkipchain) SignNewBlock(nodes []*sda.TreeNode) error {
	//create aggregate key and add it
	suite := network.Suite
	aggregatekey := suite.Point().Null()

	for i := 0; i < len(nodes); i++ {
		aggregatekey = suite.Point().Add(aggregatekey, nodes[i].Entity.Public)
	}
	block := &SkipBlock{Index: p.SkipChain[string(p.LastBlock)].Index + 1} //TODO fill the fields
	block.BackLink = append(block.BackLink, p.LastBlock)
	err := p.StartSignature(*block)
	if err != nil {
		return err
	}
	return nil
}
