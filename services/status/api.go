package status

import (
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

// Client is a structure to communicate with status service
type Client struct {
	*sda.Client
}

// NewClient makes a new Client
func NewClient() *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

// Request sends requests to all other members of network and creates client.
func (c *Client) Request(dst *network.ServerIdentity) (*Response, sda.ClientError) {
	resp := &Response{}
	cerr := c.SendProtobuf(dst, &Request{}, resp)
	if cerr != nil {
		return nil, cerr
	}
	return resp, nil
}
