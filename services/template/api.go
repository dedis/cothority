package template

/*
The api.go defines the methods that can be called from the outside. Most
of the methods will take a roster so that the service knows which nodes
it should work with.

This part of the service runs on the client or the app.
*/

import (
	"errors"

	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

// Client is a structure to communicate with the CoSi
// service
type Client struct {
	*sda.Client
}

// NewClient instantiates a new cosi.Client
func NewClient() *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

// Clock will return the time in seconds it took to run the protocol.
func (c *Client) Clock(r *sda.Roster) (time.Duration, error) {
	dst := r.RandomServerIdentity()
	log.Lvl4("Sending message to", dst)
	now := time.Now()
	reply, err := c.Send(dst, &CountRequest{})
	if e := sda.ErrMsg(reply, err); e != nil {
		return time.Duration(0), e
	}
	_, ok := reply.Msg.(CountResponse)
	if !ok {
		return time.Duration(0), errors.New("Wrong return-type.")
	}
	return time.Now().Sub(now), nil
}

// Count will return the number of times `Clock` has been called on this
// service-node.
func (c *Client) Count(si *network.ServerIdentity) (int, error) {
	reply, err := c.Send(si, &CountRequest{})
	if e := sda.ErrMsg(reply, err); e != nil {
		return -1, e
	}
	cr, ok := reply.Msg.(CountResponse)
	if !ok {
		return -1, errors.New("Wrong return-type.")
	}
	return cr.Count, nil
}
