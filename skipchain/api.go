package skipchain

import (
	"bytes"

	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// TODO:
// - change 'Propose*' to 'Add*'
// - find better names for Roster- and Data-SkipChain

const (
	// ErrorBlockNotFound indicates that for any number of operations the
	// corresponding block has not been found.
	ErrorBlockNotFound = 4100 + iota
	// ErrorBlockNoParent indicates that a parent should be there but hasn't
	// been found.
	ErrorBlockNoParent
	// ErrorBlockContent indicates that part of a block is in an invalid state.
	ErrorBlockContent
	// ErrorParameterWrong indicates that a given parameter is out of bounds.
	ErrorParameterWrong
	// ErrorVerification indicates that a given block could not be verified
	// and a signature is invalid.
	ErrorVerification
	// ErrorOnet indicates an error from the onet framework
	ErrorOnet
)

// Client is a structure to communicate with the Skipchain
// service from the outside
type Client struct {
	*onet.Client
}

// NewClient instantiates a new client with name 'n'
func NewClient() *Client {
	return &Client{Client: onet.NewClient("Skipchain")}
}

// NewLocalClient takes a LocalTest in order
func NewLocalClient(local *onet.LocalTest) *Client {
	return &Client{Client: local.NewClient("Skipchain")}
}

// CreateRootControl creates two Skipchains: a root SkipChain with
// maximumHeight of maxHRoot and an control SkipChain with
// maximumHeight of maxHControl. It connects both chains for later
// reference.
//
// Both chains will be created as 'Roster'-chains.
func (c *Client) CreateRootControl(elRoot, elControl *onet.Roster, baseHeight, maxHRoot, maxHControl int, ver VerifierID) (root, control *SkipBlock, cerr onet.ClientError) {
	log.Lvl2("Creating root roster", elRoot)
	root, cerr = c.CreateRoster(elRoot, baseHeight, maxHRoot, ver, nil)
	if cerr != nil {
		return
	}
	log.Lvl2("Creating control roster", elControl)
	control, cerr = c.CreateRoster(elControl, baseHeight, maxHControl, ver, root.Hash)
	if cerr != nil {
		return
	}
	return c.LinkParentChildBlock(root, control)
}

// CreateRoster will create a new SkipChainRoster with the parameters given
func (c *Client) CreateRoster(el *onet.Roster, baseH, maxH int, ver VerifierID, parent SkipBlockID) (*SkipBlock, onet.ClientError) {
	genesis := NewSkipBlock()
	genesis.Roster = el
	genesis.VerifierID = ver
	genesis.MaximumHeight = maxH
	genesis.BaseHeight = baseH
	genesis.ParentBlockID = parent
	sb, cerr := c.proposeSkipBlock(genesis, nil, nil)
	if cerr != nil {
		return nil, cerr
	}
	return sb.Latest, nil
}

// ProposeRoster will propose to add a new SkipBlock containing the 'roster' to
// an existing SkipChain. If it succeeds, it will return the old and the new
// SkipBlock.
func (c *Client) ProposeRoster(latest *SkipBlock, el *onet.Roster) (*ProposedSkipBlockReply, onet.ClientError) {
	return c.proposeSkipBlock(latest, el, nil)
}

// CreateData will create a new SkipChainData with the parameters given
func (c *Client) CreateData(parent *SkipBlock, baseH, maxH int, ver VerifierID, d network.Message) (
	*SkipBlock, *SkipBlock, onet.ClientError) {
	data := NewSkipBlock()
	data.MaximumHeight = maxH
	data.BaseHeight = baseH
	data.VerifierID = ver
	data.ParentBlockID = parent.Hash
	data.Roster = parent.Roster
	dataMsg, cerr := c.proposeSkipBlock(data, nil, d)
	if cerr != nil {
		return nil, nil, cerr
	}
	data = dataMsg.Latest

	return c.LinkParentChildBlock(parent, data)
}

// ProposeData will propose to add a new SkipBlock containing 'data' to an existing
// SkipChain. If it succeeds, it will return the old and the new SkipBlock.
func (c *Client) ProposeData(parent *SkipBlock, latest *SkipBlock, d network.Message) (*ProposedSkipBlockReply, onet.ClientError) {
	return c.proposeSkipBlock(latest, parent.Roster, d)
}

// LinkParentChildBlock sends a request to create a link from the parent to the
// child block and inversely. The child-block is supposed to already have
// the parentBlockID set and be accepted.
func (c *Client) LinkParentChildBlock(parent, child *SkipBlock) (*SkipBlock, *SkipBlock, onet.ClientError) {
	log.Lvl3(parent, child)
	if err := child.VerifyForwardSignatures(); err != nil {
		return nil, nil, onet.NewClientError(err)
	}
	if !bytes.Equal(parent.Hash, child.ParentBlockID) {
		return nil, nil, onet.NewClientErrorCode(ErrorBlockNoParent, "Child doesn't point to that parent")
	}
	host := parent.Roster.RandomServerIdentity()
	reply := &SetChildrenSkipBlockReply{}
	cerr := c.SendProtobuf(host, &SetChildrenSkipBlock{parent.Hash, child.Hash},
		reply)
	if cerr != nil {
		return nil, nil, cerr
	}
	return reply.Parent, reply.Child, nil
}

// GetUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain.
func (c *Client) GetUpdateChain(parent *SkipBlock, latest SkipBlockID) (reply *GetUpdateChainReply, cerr onet.ClientError) {
	log.Lvl3(parent, latest)
	h := parent.Roster.RandomServerIdentity()
	reply = &GetUpdateChainReply{}
	cerr = c.SendProtobuf(h, &GetUpdateChain{latest}, reply)
	if cerr != nil {
		return
	}
	return
}

// proposeSkipBlock sends a proposeSkipBlock to the service. If latest has
// a Nil-Hash, it will be used as a
// - rosterSkipBlock if data is nil, the Roster will be taken from 'el'
// - dataSkipBlock if data is non-nil. Furthermore 'el' will hold the activeRoster
// to send the request to.
func (c *Client) proposeSkipBlock(latest *SkipBlock, el *onet.Roster, d network.Message) (reply *ProposedSkipBlockReply, cerr onet.ClientError) {
	log.Lvlf3("%#v", latest)
	activeRoster := latest.Roster
	hash := latest.Hash
	propose := latest
	if !hash.IsNull() {
		// We have to create a new SkipBlock to propose to the
		// service
		propose = NewSkipBlock()
		if d == nil {
			// This is a RosterSkipBlock
			propose.Roster = el
		} else {
			// DataSkipBlock will be set later, just make sure that
			// there will be a receiver
			activeRoster = el
		}
	}
	if d != nil {
		// Set either a new or a proposed SkipBlock
		b, e := network.Marshal(d)
		if e != nil {
			cerr = onet.NewClientError(e)
			return
		}
		propose.Data = b
	}
	host := activeRoster.RandomServerIdentity()
	reply = &ProposedSkipBlockReply{}
	cerr = c.SendProtobuf(host, &ProposeSkipBlock{hash, propose}, reply)
	if cerr != nil {
		return
	}
	return
}
