package skipchain

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

// Client is a structure to communicate with the Skipchain
// service from the outside
type Client struct {
	*sda.Client
}

// NewClient instantiates a new client with name 'n'
func NewClient() *Client {
	return &Client{Client: sda.NewClient("Skipchain")}
}

// CreateRootInterm creates two Skipchains: a root SkipChain with
// maximumHeight of maxHRoot and an intermediate SkipChain with
// maximumHeight of maxHInterm. It connects both chains for later
// reference.
func (c *Client) CreateRootInterm(elRoot, elInter *sda.EntityList, maxHRoot, maxHInter int, ver VerifierID) (root, inter *SkipBlockRoster, err error) {
	h := elRoot.List[0]
	err = nil
	root = NewSkipBlockRoster(elRoot)
	root.MaximumHeight = maxHRoot
	root.VerifierId = ver
	inter = NewSkipBlockRoster(elInter)
	inter.MaximumHeight = maxHInter
	inter.VerifierId = ver
	rootMsg, err := c.Send(h, &ProposeSkipBlockRoster{nil, root})
	if err != nil {
		return
	}
	root = rootMsg.Msg.(ProposedSkipBlockReplyRoster).Latest
	interMsg, err := c.Send(h, &ProposeSkipBlockRoster{nil, inter})
	if err != nil {
		return
	}
	inter = interMsg.Msg.(ProposedSkipBlockReplyRoster).Latest

	replyMsg, err := c.Send(h, &SetChildrenSkipBlock{root.Hash, inter.Hash})
	if err != nil {
		return
	}
	reply := replyMsg.Msg.(SetChildrenSkipBlockReply)
	root = reply.Parent
	inter = reply.ChildRoster
	return
}

// CreateData adds a Data-chain to the given intermediate-chain using
// a maximumHeight of maxH. It will add 'data' to that chain which will
// be verified using the ver-function.
func (c *Client) CreateData(parent *SkipBlockRoster, maxH int, d network.ProtocolMessage, ver VerifierID) (data *SkipBlockData, err error) {
	h := parent.EntityList.List[0]
	data = NewSkipBlockData()
	b, err := network.MarshalRegisteredType(d)
	if err != nil {
		return
	}
	data.Data = b
	data.MaximumHeight = maxH
	data.VerifierId = ver
	data.ParentBlock = parent.Hash
	dataMsg, err := c.Send(h, &ProposeSkipBlockData{nil, data})
	if err != nil {
		return
	}
	data = dataMsg.Msg.(ProposedSkipBlockReplyData).Latest

	replyMsg, err := c.Send(h, &SetChildrenSkipBlock{parent.Hash, data.Hash})
	if err != nil {
		return
	}
	reply := replyMsg.Msg.(SetChildrenSkipBlockReply)
	*parent = *reply.Parent
	data = reply.ChildData
	return
}

// ProposeRoster will propose to add a new SkipBlock containing the 'roster' to
// an existing SkipChain. If it succeeds, it will return the old and the new
// SkipBlock
func (c *Client) ProposeRoster(latest SkipBlockID, roster *sda.EntityList) (*ProposedSkipBlockReply, error) {
	return nil, nil
}

// ProposeData will propose to add a new SkipBlock containing 'data' to an existing
// SkipChain. If it succeeds, it will return the old and the new SkipBlock.
func (c *Client) ProposeData(latest SkipBlockID, data network.ProtocolMessage) (*ProposedSkipBlockReply, error) {
	return nil, nil
}

// GetUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain.
func (c *Client) GetUpdateChain(latest SkipBlockID) (*GetUpdateChainReply, error) {
	return nil, nil
}
