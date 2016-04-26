package skipchain

import "github.com/dedis/cothority/lib/sda"

// Client is a structure to communicate with the Skipchain
// service from the outside
type Client struct {
	*sda.Client
}

// NewClient instantiates a new client with name 'n'
func NewClient() *Client {
	return &Client{Client: sda.NewClient("Skipchain")}
}

func (c *Client) ProposeSkipBlock(latest SkipBlockID, proposed SkipBlock) (*ProposedSkipBlockReply, error) {
	return nil, nil
}

func (c *Client) GetUpdateChain(latest SkipBlockID) (*GetUpdateChainReply, error) {
	return nil, nil
}
