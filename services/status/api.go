package status

import (
	"github.com/dedis/cothority/log"
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

// GetStatus Sends requests to all other members of network and creates client
func (c *Client) GetStatus(dst *network.ServerIdentity) (*Response, sda.ClientError) {
	request := &Request{}
	//send request to all entities in the network
	log.Lvl4("Sending Request to ", dst)
	reply := &Response{}
	cerr := c.SendProtobuf(dst, request, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply, nil

}
