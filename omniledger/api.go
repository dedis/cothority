package omniledger

import (
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
)

// ServiceName is used for registration on the onet.
const ServiceName = "OmniLedger"

// Client is a structure to communicate with the OmniLedger
// service.
type Client struct {
	*onet.Client
	ID     skipchain.SkipBlockID
	Roster onet.Roster
}

// NewClient returns a new client connected to the service
func NewClient(ID skipchain.SkipBlockID, Roster onet.Roster) *Client {
	return &Client{
		Client: onet.NewClient(cothority.Suite, ServiceName),
		ID:     ID,
		Roster: Roster,
	}
}

func NewOmniLedger(req *CreateOmniLedger) (*Client, *CreateOmniLedgerResponse,
	error) {
	// Create client
	c := NewClient(nil, req.Roster)

	// Create reply struct
	req.Version = byzcoin.CurrentVersion
	reply := &CreateOmniLedgerResponse{}
	err := c.SendProtobuf(req.Roster.List[0], req, reply)
	if err != nil {
		return nil, nil, err
	}

	c.ID = reply.IDSkipBlock.CalculateHash()

	return c, reply, nil
}
