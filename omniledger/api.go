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

// NewClient instantiates a new OmniLedger client.
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

func (c *Client) CreateOmniLedger(req *CreateOmniLedger) (*CreateOmniLedgerResponse, error) {
	// Create reply struct
	req.Version = byzcoin.CurrentVersion
	reply := &CreateOmniLedgerResponse{}
	err := c.SendProtobuf(c.Roster.List[0], req, reply)
	if err != nil {
		return nil, err
	}

	return reply, nil
}
