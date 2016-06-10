package skipchain

import (
	"bytes"
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

// TODO - send whole block along in 'data' for BFTSignature
//	and verify the validity of the block
// TODO - correctly convert the BFT-signature to CoSi-Signature by removing
//	the exception-field

// Client is a structure to communicate with the Skipchain
// service from the outside
type Client struct {
	*sda.Client
}

// NewClient instantiates a new client with name 'n'
func NewClient() *Client {
	return &Client{Client: sda.NewClient("Skipchain")}
}

// CreateRootControl creates two Skipchains: a root SkipChain with
// maximumHeight of maxHRoot and an control SkipChain with
// maximumHeight of maxHControl. It connects both chains for later
// reference.
func (c *Client) CreateRootControl(elRoot, elControl *sda.EntityList, baseHeight, maxHRoot, maxHControl int, ver VerifierID) (root, control *SkipBlock, err error) {
	dbg.Lvl2("Creating root roster")
	root, err = c.CreateRoster(elRoot, baseHeight, maxHRoot, ver, nil)
	if err != nil {
		return
	}
	dbg.Lvl2("Creating control roster")
	control, err = c.CreateRoster(elControl, baseHeight, maxHControl, ver, root.Hash)
	if err != nil {
		return
	}
	return c.LinkParentChildBlock(root, control)
}

// ProposeRoster will propose to add a new SkipBlock containing the 'roster' to
// an existing SkipChain. If it succeeds, it will return the old and the new
// SkipBlock.
func (c *Client) ProposeRoster(latest *SkipBlock, el *sda.EntityList) (reply *ProposedSkipBlockReply, err error) {
	return c.proposeSkipBlock(latest, el, nil)
}

// CreateRoster will create a new SkipChainRoster with the parameters given
func (c *Client) CreateRoster(el *sda.EntityList, baseH, maxH int, ver VerifierID, parent SkipBlockID) (*SkipBlock, error) {
	genesis := NewSkipBlock()
	genesis.EntityList = el
	genesis.VerifierID = ver
	genesis.MaximumHeight = maxH
	genesis.BaseHeight = baseH
	genesis.ParentBlockID = parent
	sb, err := c.proposeSkipBlock(genesis, nil, nil)
	if err != nil {
		return nil, err
	}
	return sb.Latest, nil
}

// ProposeData will propose to add a new SkipBlock containing 'data' to an existing
// SkipChain. If it succeeds, it will return the old and the new SkipBlock.
func (c *Client) ProposeData(parent *SkipBlock, latest *SkipBlock, d network.ProtocolMessage) (reply *ProposedSkipBlockReply, err error) {
	return c.proposeSkipBlock(latest, parent.EntityList, d)
}

// CreateData will create a new SkipChainData with the parameters given
func (c *Client) CreateData(parent *SkipBlock, baseH, maxH int, ver VerifierID, d network.ProtocolMessage) (
	*SkipBlock, *SkipBlock, error) {
	data := NewSkipBlock()
	data.MaximumHeight = maxH
	data.BaseHeight = baseH
	data.VerifierID = ver
	data.ParentBlockID = parent.Hash
	data.EntityList = parent.EntityList
	dataMsg, err := c.proposeSkipBlock(data, nil, d)
	if err != nil {
		return nil, nil, err
	}
	data = dataMsg.Latest

	return c.LinkParentChildBlock(parent, data)
}

// LinkParentChildBlock sends a request to create a link from the parent to the
// child block and inversely. The child-block is supposed to already have
// the parentBlockID set and be accepted.
func (c *Client) LinkParentChildBlock(parent, child *SkipBlock) (*SkipBlock, *SkipBlock, error) {
	if err := child.VerifySignatures(); err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(parent.Hash, child.ParentBlockID) {
		return nil, nil, errors.New("Child doesn't point to that parent")
	}
	host := parent.EntityList.GetRandom()
	replyMsg, err := c.Send(host, &SetChildrenSkipBlock{parent.Hash, child.Hash})
	if err != nil {
		return nil, nil, err
	}
	reply := replyMsg.Msg.(SetChildrenSkipBlockReply)
	return reply.Parent, reply.Child, nil
}

// GetUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain.
func (c *Client) GetUpdateChain(parent *SkipBlock, latest SkipBlockID) (reply *GetUpdateChainReply, err error) {
	h := parent.EntityList.GetRandom()
	r, err := c.Send(h, &GetUpdateChain{latest})
	if err != nil {
		return
	}
	replyVal := r.Msg.(GetUpdateChainReply)
	reply = &replyVal
	return
}

// proposeSkipBlock sends a proposeSkipBlock to the service. If latest has
// a Nil-Hash, it will be used as a
// - rosterSkipBlock if data is nil, the EntityList will be taken from 'el'
// - dataSkipBlock if data is non-nil. Furthermore 'el' will hold the activeRoster
// to send the request to.
func (c *Client) proposeSkipBlock(latest *SkipBlock, el *sda.EntityList, d network.ProtocolMessage) (reply *ProposedSkipBlockReply, err error) {
	activeRoster := latest.EntityList
	hash := latest.Hash
	propose := latest
	if !hash.IsNull() {
		// We have to create a new SkipBlock to propose to the
		// service
		propose = NewSkipBlock()
		if d == nil {
			// This is a RosterSkipBlock
			propose.EntityList = el
		} else {
			// DataSkipBlock will be set later, just make sure that
			// there will be a receiver
			activeRoster = el
		}
	}
	if d != nil {
		// Set either a new or a proposed SkipBlock
		var b []byte
		b, err = network.MarshalRegisteredType(d)
		if err != nil {
			return
		}
		propose.Data = b
	}
	host := activeRoster.GetRandom()
	r, err := c.Send(host, &ProposeSkipBlock{hash, propose})
	if err != nil {
		return
	}
	replyVal := r.Msg.(ProposedSkipBlockReply)
	reply = &replyVal
	return
}
