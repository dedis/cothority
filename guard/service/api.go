package guard

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// Client is a structure to communicate with Guard service
type Client struct {
	*onet.Client
}

// NewClient makes a new Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(ServiceName)}
}

// SendToGuard is the function that sends a request to the guard server from the client and receives the responses
func (c *Client) SendToGuard(dst *network.ServerIdentity, UID []byte, epoch []byte, t abstract.Point) (*Response, onet.ClientError) {
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
