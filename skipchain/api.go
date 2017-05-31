package skipchain

import (
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// ServiceName is used to register this service to onet.
const ServiceName = "Skipchain"

// Client is a structure to communicate with the Skipchain
// service from the outside
type Client struct {
	*onet.Client
}

// NewClient instantiates a new client with name 'n'
func NewClient() *Client {
	return &Client{Client: onet.NewClient(ServiceName)}
}

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

// Basic methods for skipchains

// StoreSkipBlock asks the cothority to store the given skipblock to the
// appropriate service. The SkipBlock must be correctly filled out:
//  - GenesisID: if this is nil, a new skipchain will be generated
//  - Roster: needs to be non-nil, but can be copied from previous block
//  - MaximumHeight, BaseHeight, VerifierIDs: for a new skipchain, they need
//    to be initialised
func (c *Client) StoreSkipBlock(sb *SkipBlock) (*SkipBlock, onet.ClientError) {
	reply := &StoreSkipBlockReply{}
	cerr := c.SendProtobuf(sb.Roster.RandomServerIdentity(), &StoreSkipBlock{sb}, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply.Latest, nil
}

// GetBlocks returns a list of blocks:
//  - roster is the roster of nodes who should know about the blocks in question
//  - start: the first block you want to retrieve - if it is nil, end _must_ be given
//  - end: if it is nil, create a list up to the latest block. If start is nil,
//    only return that block. If both start and end are given, return those two blocks
//    and all blocks in between.
//  - max: the maximum height. If you have a multi-level skipchain, you can set
//    max := 1 and get _all_ skipblocks from start to end. If max == 0, return
//    only the shortest skipchain from start to end (or the latest, if end == nil).
func (c *Client) GetBlocks(roster *onet.Roster, start, end SkipBlockID, max int) ([]*SkipBlock, onet.ClientError) {
	if roster == nil {
		return nil, onet.NewClientErrorCode(ErrorParameterWrong, "No roster given")
	}
	if start == nil && end == nil {
		return nil, onet.NewClientErrorCode(ErrorParameterWrong, "Start and/or end must be given")
	}
	reply := &GetBlocksReply{}
	cerr := c.SendProtobuf(roster.RandomServerIdentity(), &GetBlocks{start, end, max}, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply.Reply, nil
}

// GetAllSkipchains returns all skipchains known to that conode. If none are
// known, an empty slice is returned.
func (c *Client) GetAllSkipchains(si *network.ServerIdentity) (reply *GetAllSkipchainsReply,
	cerr onet.ClientError) {
	reply = &GetAllSkipchainsReply{}
	cerr = c.SendProtobuf(si, &GetAllSkipchains{}, reply)
	return
}

// Combined and convenience methods for skipchains

// CreateGenesis is a convenience function to create a new SkipChain with the
// given parameters.
//  - r is the responsible roster
//  - baseH is the base-height - the distance between two non-height-1 skipblocks
//  - maxH is the maximum height, which must be <= baseH
//  - ver is a slice of verifications to apply to that block
//  - data can be nil or any data that will be network.Marshaled to the skipblock
//  - parent is the responsible parent-block, can be 'nil'
//
// This function returns the created skipblock or nil and an error.
func (c *Client) CreateGenesis(r *onet.Roster, baseH, maxH int, ver []VerifierID,
	data interface{}, parent SkipBlockID) (*SkipBlock, onet.ClientError) {
	genesis := NewSkipBlock()
	genesis.Roster = r
	genesis.VerifierIDs = ver
	genesis.MaximumHeight = maxH
	genesis.BaseHeight = baseH
	genesis.ParentBlockID = parent
	if err := genesis.SetData(data); err != nil {
		return nil, onet.NewClientErrorCode(ErrorParameterWrong, err.Error())
	}
	if data != nil {
	}
	_, sb, cerr := c.AddSkipBlock(genesis, nil, nil)
	if cerr != nil {
		return nil, cerr
	}
	return sb, nil
}

// AddSkipBlock asks the cothority to store the new skipblock, and eventually
// attach it to the 'latest' skipblock.
//  - latest is the skipblock where the new skipblock is appended. If r and d
//   are nil, a new skipchain will be created with 'latest' as genesis-block.
//  - r is the new roster for that block. If r is nil, the previous roster
//   will be used.
//  - d is the data for the new block. It can be nil. If it is not of type
//   []byte, it will be marshalled using `network.Marshal`.
//
// If you need to change the parent, you have to use StoreSkipBlock
func (c *Client) AddSkipBlock(latest *SkipBlock, r *onet.Roster,
	data network.Message) (previousSB, latestSB *SkipBlock, cerr onet.ClientError) {
	log.Lvlf3("%#v", latest)
	var newBlock *SkipBlock
	if r == nil && data == nil {
		newBlock = latest
	} else {
		newBlock = latest.Copy()
		if r != nil {
			newBlock.Roster = r
		}
		if err := newBlock.SetData(data); err != nil {
			return nil, nil, onet.NewClientErrorCode(ErrorParameterWrong, err.Error())
		}
		newBlock.Index = latest.Index + 1
		newBlock.GenesisID = latest.SkipChainID()
	}
	host := latest.Roster.RandomServerIdentity()
	reply := &StoreSkipBlockReply{}
	cerr = c.SendProtobuf(host, &StoreSkipBlock{newBlock}, reply)
	if cerr != nil {
		return
	}
	previousSB = reply.Previous
	latestSB = reply.Latest
	return
}

// GetUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain. It takes a roster that knows the
// 'latest' skipblock and the id (=hash) of the latest skipblock.
func (c *Client) GetUpdateChain(roster *onet.Roster, latest SkipBlockID) ([]*SkipBlock, onet.ClientError) {
	return c.GetBlocks(roster, latest, nil, 0)
}

// GetFlatUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain. It takes a roster that knows the
// 'latest' skipblock and the id (=hash) of the latest skipblock. Instead of
// returning the shortest chain following the forward-links, it returns the
// chain with maximum-height == 1.
func (c *Client) GetFlatUpdateChain(roster *onet.Roster, latest SkipBlockID) ([]*SkipBlock, onet.ClientError) {
	return c.GetBlocks(roster, latest, nil, 1)
}

// GetSingleBlock searches for a block with the given ID and returns that block,
// or an error if that block is not found.
func (c *Client) GetSingleBlock(roster *onet.Roster, id SkipBlockID) (*SkipBlock, onet.ClientError) {
	sbs, cerr := c.GetBlocks(roster, nil, id, 0)
	if cerr != nil {
		return nil, cerr
	}
	return sbs[0], nil
}

// BunchAddBlock adds a block to the latest block from the bunch. If the block
// doesn't have a roster set, it will be copied from the last block.
func (c *Client) BunchAddBlock(bunch *SkipBlockBunch, r *onet.Roster, data interface{}) (*SkipBlock, onet.ClientError) {
	_, sbNew, err := c.AddSkipBlock(bunch.Latest, r, data)
	if err != nil {
		return nil, err
	}
	id := bunch.Store(sbNew)
	if id == nil {
		return nil, onet.NewClientErrorCode(ErrorVerification,
			"Couldn't add block to bunch")
	}
	return sbNew, nil
}

// BunchUpdate contacts the nodes and asks for an update of the chains available
// in the bunch.
func (c *Client) BunchUpdate(bunch *SkipBlockBunch) onet.ClientError {
	sbs, cerr := c.GetUpdateChain(bunch.Latest.Roster, bunch.Latest.Hash)
	if cerr != nil {
		return cerr
	}
	if len(sbs) > 1 {
		for _, sb := range sbs {
			bunch.Store(sb)
		}
	}
	return nil
}
