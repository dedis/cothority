package service

import (
	"github.com/dedis/cothority"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// Client is a structure to communicate with the randshare service
type Client struct {
	*onet.Client
}

// NewClient instantiates a new randshare.Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// RandShareRequest sends request
func (c *Client) RandShareRequest(r *onet.Roster, purpose []byte) (network.Message, error) {
	//TODO
}
