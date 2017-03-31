package skipchain

import (
	"encoding/hex"
	"io/ioutil"
	"net/http"

	"gopkg.in/dedis/crypto.v0/abstract"
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

// StoreSkipBlock asks the cothority to store the new skipblock, and eventually
// attach it to the 'latest' skipblock.
//  - latest is the skipblock where the new skipblock is appended. If el and d
//   are nil, a new skipchain will be created with 'latest' as genesis-block.
//  - el is the new roster for that block. If el is nil, the previous roster
//   will be used.
//  - d is the data for the new block. It can be nil.
func (c *Client) StoreSkipBlock(latest *SkipBlock, el *onet.Roster, d network.Message) (reply *StoreSkipBlockReply, cerr onet.ClientError) {
	log.Lvlf3("%#v", latest)
	var newBlock *SkipBlock
	var latestID SkipBlockID
	if el == nil && d == nil {
		newBlock = latest
	} else {
		newBlock = latest.Copy()
		if el != nil {
			newBlock.Roster = el
		}
		if d != nil {
			buf, err := network.Marshal(d)
			if err != nil {
				return nil, onet.NewClientErrorCode(ErrorParameterWrong,
					"Couldn't marshal data: "+err.Error())
			}
			newBlock.Data = buf
		}
		latestID = latest.Hash
	}
	host := latest.Roster.RandomServerIdentity()
	reply = &StoreSkipBlockReply{}
	cerr = c.SendProtobuf(host, &StoreSkipBlock{latestID, newBlock}, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply, nil
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
	sb, cerr := c.StoreSkipBlock(genesis, nil, nil)
	if cerr != nil {
		return nil, cerr
	}
	return sb.Latest, nil
}

// CreateRootControl is a convenience function and creates two Skipchains:
// a root SkipChain with maximumHeight of maxHRoot and a control SkipChain with
// maximumHeight of maxHControl. It connects both chains for later
// reference. The root-chain will use `VerificationRoot` and the config-chain
// will use `VerificationConfig`.
//
// A slice of verification-functions is given for the root and the control
// skipchain.
func (c *Client) CreateRootControl(elRoot, elControl *onet.Roster,
	keys []abstract.Point, baseHeight,
	maxHRoot, maxHControl int) (root, control *SkipBlock, cerr onet.ClientError) {
	log.Lvl2("Creating root roster", elRoot)
	root, cerr = c.CreateGenesis(elRoot, baseHeight, maxHRoot,
		VerificationRoot, nil, nil)
	if cerr != nil {
		return
	}
	log.Lvl2("Creating control roster", elControl)
	control, cerr = c.CreateGenesis(elControl, baseHeight, maxHControl,
		VerificationControl, nil, root.Hash)
	if cerr != nil {
		return
	}
	return root, control, cerr
}

// FindSkipChain takes the ID of a skipchain and an optional URL for finding the
// appropriate skipchain. If no URL is given, the default
// "http://skipchain.dedis.ch" is used. If successful, it will return the latest
// known block.
func (c *Client) FindSkipChain(id SkipBlockID, url string) (*SkipBlock, error) {
	if url == "" {
		url = "http://skipchain.dedis.ch"
	}
	resp, err := http.Get(url + "/" + hex.EncodeToString(id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Print(body)
	return nil, nil
}

// GetUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain. It takes a roster that knows the
// 'latest' skipblock and the id (=hash) of the latest skipblock.
func (c *Client) GetUpdateChain(roster *onet.Roster, latest SkipBlockID) (reply *GetUpdateChainReply, cerr onet.ClientError) {
	reply = &GetUpdateChainReply{}
	cerr = c.SendProtobuf(roster.RandomServerIdentity(),
		&GetUpdateChain{latest}, reply)
	return
}
