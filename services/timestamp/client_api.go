package timestamp

import (
	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

// Client is a structure to communicate with the CoSi
// service
type Client struct {
	*sda.Client
}

// NewClient instantiates a new Timestamp client
func NewClient() *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

// SignMsg sends a CoSi sign request to the Cothority defined by the given
// Roster
func (c *Client) SignMsg(root *network.ServerIdentity, msg []byte) (*SignatureResponse, error) {
	serviceReq := &SignatureRequest{
		Message: msg,
	}
	log.Lvl4("Sending message to", root)
	reply, err := c.Send(root, serviceReq)
	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}
	sr, ok := reply.Msg.(SignatureResponse)
	if !ok {
		return nil, errors.New("This is odd: couldn't cast reply.")
	}
	return &sr, nil
}
