package status

import (
	"github.com/dedis/cothority"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// Client is a structure to communicate with status service
type Client struct {
	*onet.Client
}

// NewClient makes a new Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(ServiceName, cothority.Suite)}
}

// Request sends requests to all other members of network and creates client.
func (c *Client) Request(dst *network.ServerIdentity) (*Response, onet.ClientError) {
	resp := &Response{}
	cerr := c.SendProtobuf(dst, &Request{}, resp)
	if cerr != nil {
		return nil, cerr
	}
	return resp, nil
}
