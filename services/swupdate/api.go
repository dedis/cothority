package swupdate

import (
	"errors"

	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
)

// Client is a structure to communicate with the software-update service.
type Client struct {
	*sda.Client
	Roster *sda.Roster
	ProjectID
	Root *network.ServerIdentity
}

// NewClient instantiates a new communication with the swupdate-client.
func NewClient(r *sda.Roster) *Client {
	return &Client{
		Client: sda.NewClient(ServiceName),
		Roster: r,
		Root:   r.List[0],
	}
}

func (c *Client) LatestUpdates(latestIDs []skipchain.SkipBlockID) (*LatestBlocksRet, error) {
	lbs := &LatestBlocks{latestIDs}
	p, err := c.Send(c.Root, lbs)
	if err != nil {
		return nil, err
	}
	lbr, ok := p.Msg.(*LatestBlocksRet)
	if !ok {
		return nil, errors.New("Wrong message")
	}
	return lbr, nil
}

func (c *Client) TimestampRequests(names []string) (*TimestampRets, error) {
	t := &TimestampRequests{names}
	r, err := c.Send(c.Root, t)
	if err != nil {
		return nil, err
	}
	tr, ok := r.Msg.(*TimestampRets)
	if !ok {
		return nil, errors.New("Wrong Message")
	}
	return tr, nil
}
