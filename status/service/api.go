package status

import (
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/network"
)

// Client is a structure to communicate with status service
type Client struct {
	*onet.Client
}

// NewClient makes a new Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// Request sends requests to all other members of network and creates client.
func (c *Client) Request(dst *network.ServerIdentity) (*Response, error) {
	resp := &Response{}
	err := c.SendProtobuf(dst, &Request{}, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
