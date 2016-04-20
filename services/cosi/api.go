package cosi

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

// SkipchainClient is a structure to communicate with the Skipchain
// service from the outside
type Client struct {
	*sda.Client
}

// NewSkipchainClient instantiates a new client with name 'n'
func NewClient() *Client {
	return &Client{Client: sda.NewClient("Skipchain")}
}

// SendActiveAdd takes a previous and a new skipchain and sends it to the
// first TreeNodeEntity
func (c *Client) SignMsg(el *sda.EntityList, msg []byte) (*ServiceResponse, error) {
	dbg.LLvl3("Starting signing-request", new)
	serviceReq := &ServiceRequest{
		EntityList: el,
		Message:    msg,
	}
	dst := el.List[0]
	dbg.LLvl4("Sending message to", dst)
	reply, err := c.Send(dst, serviceReq)
	if err != nil {
		return nil, err
	}
	sr, ok := reply.Msg.(ServiceResponse)
	if !ok {
		return nil, sda.ErrMsg(sr, err)
	}
	return &sr, nil
}
