package status

import (
	"errors"

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
func (c *Client) GetStatus(dst *network.ServerIdentity) (*Response, error) {
	ServiceReq := &Request{}
	//send request to all entities in the network
	log.Lvl4("Sending Request to ", dst)
	reply, err := c.Send(dst, ServiceReq)
	if e := network.ErrMsg(reply, err); e != nil {
		return nil, e
	}
	sr, ok := reply.Msg.(Response)
	if !ok {
		return nil, errors.New("Wrong return type")
	}
	return &sr, nil

}
