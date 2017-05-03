package skipchain

import (
	"errors"

	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
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

// AddSkipBlock asks the cothority to store the new skipblock, and eventually
// attach it to the 'latest' skipblock.
//  - latest is the skipblock where the new skipblock is appended. If el and d
//   are nil, a new skipchain will be created with 'latest' as genesis-block.
//  - el is the new roster for that block. If el is nil, the previous roster
//   will be used.
//  - d is the data for the new block. It can be nil. If it is not of type
//   []byte, it will be marshalled using `network.Marshal`.
//
// If you need to change the parent, you have to use StoreSkipBlock
func (c *Client) AddSkipBlock(latest *SkipBlock, el *onet.Roster,
	d network.Message) (reply *StoreSkipBlockReply, cerr onet.ClientError) {
	log.Lvlf3("%#v", latest)
	var newBlock *SkipBlock
	if el == nil && d == nil {
		newBlock = latest
	} else {
		newBlock = latest.Copy()
		if el != nil {
			newBlock.Roster = el
		}
		if d != nil {
			var ok bool
			newBlock.Data, ok = d.([]byte)
			if !ok {
				buf, err := network.Marshal(d)
				if err != nil {
					return nil, onet.NewClientErrorCode(ErrorParameterWrong,
						"Couldn't marshal data: "+err.Error())
				}
				newBlock.Data = buf
			}
		}
		newBlock.Index = latest.Index + 1
		newBlock.GenesisID = latest.SkipChainID()
	}
	host := latest.Roster.RandomServerIdentity()
	reply = &StoreSkipBlockReply{}
	cerr = c.SendProtobuf(host, &StoreSkipBlock{newBlock}, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply, nil
}

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

// CreateGenesis is a convenience function to create a new SkipChain with the
// given parameters.
//  - el is the responsible roster
//  - baseH is the base-height - the distance between two non-height-1 skipblocks
//  - maxH is the maximum height, which must be <= baseH
//  - ver is a slice of verifications to apply to that block
//  - data can be nil or any data that will be network.Marshaled to the skipblock
//  - parent is the responsible parent-block, can be 'nil'
//
// This function returns the created skipblock or nil and an error.
func (c *Client) CreateGenesis(el *onet.Roster, baseH, maxH int, ver []VerifierID,
	data interface{}, parent SkipBlockID) (*SkipBlock, onet.ClientError) {
	genesis := NewSkipBlock()
	genesis.Roster = el
	genesis.VerifierIDs = ver
	genesis.MaximumHeight = maxH
	genesis.BaseHeight = baseH
	genesis.ParentBlockID = parent
	if data != nil {
		buf, err := network.Marshal(data)
		if err != nil {
			return nil, onet.NewClientErrorCode(ErrorParameterWrong,
				err.Error())
		}
		genesis.Data = buf
	}
	sb, cerr := c.AddSkipBlock(genesis, nil, nil)
	if cerr != nil {
		return nil, cerr
	}
	return sb.Latest, nil
}

// GetUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain. It takes a roster that knows the
// 'latest' skipblock and the id (=hash) of the latest skipblock.
func (c *Client) GetUpdateChain(roster *onet.Roster, latest SkipBlockID) (reply *GetBlocksReply, cerr onet.ClientError) {
	reply = &GetBlocksReply{}
	cerr = c.SendProtobuf(roster.RandomServerIdentity(),
		&GetBlocks{latest, nil, 0}, reply)
	return
}

// GetFlatUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain. It takes a roster that knows the
// 'latest' skipblock and the id (=hash) of the latest skipblock. Instead of
// returning the shortest chain following the forward-links, it returns the
// chain with maximum-height == 1.
func (c *Client) GetFlatUpdateChain(roster *onet.Roster, latest SkipBlockID) (reply *GetBlocksReply, cerr onet.ClientError) {
	reply = &GetBlocksReply{}
	cerr = c.SendProtobuf(roster.RandomServerIdentity(),
		&GetBlocks{latest, nil, 1}, reply)
	return
}

// GetAllSkipchains returns all skipchains known to that conode. If none are
// known, an empty slice is returned.
func (c *Client) GetAllSkipchains(si *network.ServerIdentity) (reply *GetAllSkipchainsReply,
	cerr onet.ClientError) {
	reply = &GetAllSkipchainsReply{}
	cerr = c.SendProtobuf(si, &GetAllSkipchains{}, reply)
	return
}

// GetSingleBlock searches for a block with the given ID and returns that block,
// or an error if that block is not found.
func (c *Client) GetSingleBlock(roster *onet.Roster, id SkipBlockID) (*SkipBlock, onet.ClientError) {
	reply := &GetBlocksReply{}
	cerr := c.SendProtobuf(roster.RandomServerIdentity(),
		&GetBlocks{nil, id, 0}, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply.Reply[0], nil
}

// BunchAddBlock adds a block to the latest block from the bunch. If the block
// doesn't have a roster set, it will be copied from the last block.
func (c *Client) BunchAddBlock(bunch *SkipBlockBunch, sb *SkipBlock) (*SkipBlock, onet.ClientError) {
	sbNew, err := c.AddSkipBlock(bunch.Latest, sb.Roster, sb.Data)
	if err != nil {
		return nil, err
	}
	id := bunch.Store(sbNew.Latest)
	if id == nil {
		return nil, onet.NewClientErrorCode(ErrorVerification,
			"Couldn't add block to bunch")
	}
	return sb, nil
}

// BunchUpdate contacts the nodes and asks for an update of the chains available
// in the bunch.
func (c *Client) BunchUpdate(bunch *SkipBlockBunch) onet.ClientError {
	reply := &GetBlocksReply{}
	cerr := c.SendProtobuf(bunch.Latest.Roster.RandomServerIdentity(),
		&GetBlocks{bunch.Latest.Hash, nil, 0}, reply)
	if cerr != nil {
		return cerr
	}
	if len(reply.Reply) > 1 {
		for _, sb := range reply.Reply {
			bunch.Store(sb)
		}
	}
	return nil
}

// FindSkipChain takes the ID of a skipchain and an optional URL for finding the
// appropriate service. If no URL is given, the default
// "http://service.dedis.ch" is used. If successful, it will return the latest
// known block.
func FindSkipChain(id SkipBlockID, url string) (*SkipBlock, error) {
	c := NewClient()
	reply := &GetBlocksReply{}
	sid := network.NewServerIdentity(nil, "localhost:2002")
	cerr := c.SendProtobuf(sid,
		&GetBlocks{nil, id, 0}, reply)
	if cerr != nil {
		return nil, cerr
	}
	if len(reply.Reply) == 0 {
		return nil, errors.New("Didn't find skipblock")
	}
	return reply.Reply[len(reply.Reply)-1], nil
	//if url == "" {
	//	url = "http://service.dedis.ch"
	//}
	//resp, err := http.Get(url + "/" + hex.EncodeToString(id))
	//if err != nil {
	//	return nil, err
	//}
	//defer resp.Body.Close()
	//body, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	return nil, err
	//}
	//log.Print(body)
	//return nil, nil
}
