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

// CreateRootInter creates two Skipchains: a root SkipChain with
// maximumHeight of maxHRoot and an intermediate SkipChain with
// maximumHeight of maxHInter. It connects both chains for later
// reference.
func (c *Client) CreateRootInter(elRoot, elInter *sda.EntityList, maxHRoot, maxHInter int, ver VerifierID) (root, inter *SkipBlock, err error) {
	h := elRoot.List[0]
	err = nil
	root = NewSkipBlock()
	root.EntityList = elRoot
	root.MaximumHeight = maxHRoot
	root.VerifierID = ver
	inter = NewSkipBlock()
	inter.EntityList = elInter
	inter.MaximumHeight = maxHInter
	inter.VerifierID = ver
	rootMsg, err := c.Send(h, &ProposeSkipBlock{nil, root})
	if err != nil {
		return
	}
	root = rootMsg.Msg.(ProposedSkipBlockReply).Latest
	interMsg, err := c.Send(h, &ProposeSkipBlock{nil, inter})
	if err != nil {
		return
	}
	inter = interMsg.Msg.(ProposedSkipBlockReply).Latest

	replyMsg, err := c.Send(h, &SetChildrenSkipBlock{root.Hash, inter.Hash})
	if err != nil {
		return
	}
	reply := replyMsg.Msg.(SetChildrenSkipBlockReply)
	root = reply.Parent
	inter = reply.Child
	return
}

// CreateData adds a Data-chain to the given intermediate-chain using
// a maximumHeight of maxH. It will add 'data' to that chain which will
// be verified using the ver-function.
func (c *Client) CreateData(parent *SkipBlock, maxH int, d network.ProtocolMessage, ver VerifierID) (data *SkipBlock, err error) {
	h := parent.EntityList.List[0]
	data = NewSkipBlock()
	b, err := network.MarshalRegisteredType(d)
	if err != nil {
		return
	}
	data.Data = b
	data.MaximumHeight = maxH
	data.VerifierID = ver
	data.ParentBlockID = parent.Hash
	data.EntityList = parent.EntityList
	dataMsg, err := c.Send(h, &ProposeSkipBlock{nil, data})
	if err != nil {
		return
	}
	data = dataMsg.Msg.(ProposedSkipBlockReply).Latest

	replyMsg, err := c.Send(h, &SetChildrenSkipBlock{parent.Hash, data.Hash})
	if err != nil {
		return
	}
	reply := replyMsg.Msg.(SetChildrenSkipBlockReply)
	*parent = *reply.Parent
	data = reply.Child
	return
}

// ProposeRoster will propose to add a new SkipBlock containing the 'roster' to
// an existing SkipChain. If it succeeds, it will return the old and the new
// SkipBlock
func (c *Client) ProposeRoster(latest *SkipBlock, el *sda.EntityList) (reply *ProposedSkipBlockReply, err error) {
	h := latest.EntityList.List[0]
	roster := NewSkipBlock()
	roster.EntityList = el
	r, err := c.Send(h, &ProposeSkipBlock{latest.Hash, roster})
	if err != nil {
		return
	}
	replyVal := r.Msg.(ProposedSkipBlockReply)
	reply = &replyVal
	return
}

// ProposeData will propose to add a new SkipBlock containing 'data' to an existing
// SkipChain. If it succeeds, it will return the old and the new SkipBlock.
func (c *Client) ProposeData(parent *SkipBlock, latest *SkipBlock, d network.ProtocolMessage) (reply *ProposedSkipBlockReply, err error) {
	h := parent.EntityList.List[0]
	data := NewSkipBlock()
	b, err := network.MarshalRegisteredType(d)
	if err != nil {
		return
	}
	data.Data = b
	data.EntityList = parent.EntityList
	r, err := c.Send(h, &ProposeSkipBlock{latest.Hash, data})
	replyVal := r.Msg.(ProposedSkipBlockReply)
	reply = &replyVal
	return
}

// GetUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain.
func (c *Client) GetUpdateChain(parent *SkipBlock, latest SkipBlockID) (reply *GetUpdateChainReply, err error) {
	h := parent.EntityList.List[0]
	r, err := c.Send(h, &GetUpdateChain{latest})
	if err != nil {
		return
	}
	replyVal := r.Msg.(GetUpdateChainReply)
	reply = &replyVal
	return
}
