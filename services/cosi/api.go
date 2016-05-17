package cosi

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
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

// SignMsg sends a CoSi sign request to the Cothority defined by the given
// EntityList
func (c *Client) SignMsg(el *sda.EntityList, msg []byte) (*SignatureResponse, error) {
	serviceReq := &SignatureRequest{
		EntityList: el,
		Message:    msg,
	}
	dst := el.List[0]
	dbg.Lvl4("Sending message to", dst)
	reply, err := c.Send(dst, serviceReq)
	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}
	sr, ok := reply.Msg.(SignatureResponse)
	if !ok {
		return nil, errors.New("This is odd: couldn't cast reply.")
	}
	return &sr, nil
}
