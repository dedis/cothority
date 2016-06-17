package status

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

//Client is a structure to communicate with status service
type Client struct {
	*sda.Client
}

func NewClient() *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

//Sends requests to all other members of network and creates client
func (c *Client) GetStatus(dst *network.Entity) (*StatusResponse, error) {
	ServiceReq := &StatusRequest{}
	//send request to all entities in the network
	dbg.Lvl4("Sending Request to ", dst)
	reply, err := c.Send(dst, ServiceReq)
	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}
	sr, ok := reply.Msg.(StatusResponse)
	if !ok {
		return nil, errors.New("Wrong return type:")
	}
	return &sr, nil

}
