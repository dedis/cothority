package guard

import (
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

// Client is a structure to communicate with Guard service
type Client struct {
	*sda.Client
}

// NewClient makes a new Client
func NewClient() *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

// SendToGuard is the function that sends a request to the guard server from the client and receives the responses
func (c *Client) SendToGuard(dst *network.ServerIdentity, UID []byte, epoch []byte, t abstract.Point) (*Response, sda.ClientError) {
	//send request an entity in the network
	log.Lvl4("Sending Request to ", dst)
	serviceReq := &Request{UID, epoch, t}
	reply := &Response{}
	cerr := c.SendProtobuf(dst, serviceReq, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply, nil
}
