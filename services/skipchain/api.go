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
func (c *Client) CreateRootInterm(maxHRoot, maxHInterm int, ver VerifierID) (root, interm *SkipBlockRoster, err error) {
	return nil, nil, nil

}

// CreateData adds a Data-chain to the given intermediate-chain using
// a maximumHeight of maxH. It will add 'data' to that chain which will
// be verified using the ver-function.
func (c *Client) CreateData(interm *SkipBlockRoster, maxH int, data network.ProtocolMessage, ver VerifierID) (*SkipBlockData, error) {
	return nil, nil
}

func (c *Client) ProposeRoster(latest SkipBlockID, proposed SkipBlock) (*ProposedSkipBlockReply, error) {
	return nil, nil
}

func (c *Client) ProposeData(latest SkipBlockID, data network.ProtocolMessage) (*SkipBlockData, error) {
	return nil, nil
}

func (c *Client) GetUpdateChain(latest SkipBlockID) (*GetUpdateChainReply, error) {
	return nil, nil
}
