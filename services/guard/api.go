package guard

import (
	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

// Client is a structure to communicate with Guard service
type Client struct {
	*sda.Client
}

// NewClient makes a new Client
func NewClient() *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

// GetGuard is the function that sends a request to the guard server and creates a client to receive the responses
func (c *Client) GetGuard(dst *network.ServerIdentity, UID []byte, epoch []byte, t []byte) (*Response, error) {
	//send request an entity in the network
	log.Lvl4("Sending Request to ", dst)
	ServiceReq := &Request{UID, epoch, []byte(t)}
	reply, err := c.Send(dst, ServiceReq)
	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}
	sr, ok := reply.Msg.(Response)
	if !ok {
		return nil, errors.New("Wrong return type")
	}
	return &sr, nil
}
