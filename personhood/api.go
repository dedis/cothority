package personhood

// api for personhood - very minimalistic for the moment, as most of the
// calls are made from javascript.

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
)

// Client is a structure to communicate with the personhood
// service
type Client struct {
	*onet.Client
}

// NewClient instantiates a new personhood.Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// LinkPoP sends a party description to the message server for further
// reference in messages.
func (c *Client) LinkPoP(si *network.ServerIdentity, p Party) error {
	err := c.SendProtobuf(si, &LinkPoP{p}, nil)
	if err != nil {
		return err
	}
	return nil
}
