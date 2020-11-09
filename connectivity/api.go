package connectivity

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
	"golang.org/x/xerrors"
)

type Client struct {
	*onet.Client
}

func NewClient() *Client {
	return &Client{
		Client: onet.NewClient(cothority.Suite, Name),
	}
}

func (c *Client) Check(dst *network.ServerIdentity, r *onet.Roster) (*CheckReply, error) {
	reply := &CheckReply{}
	err := c.SendProtobuf(dst, &CheckRequest{Roster: r}, reply)
	if err != nil {
		return nil, xerrors.Errorf("failed to send CheckRequest: %v", err)
	}
	return reply, nil
}
