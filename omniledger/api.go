package omniledger

import (
	"github.com/dedis/cothority"
	"github.com/dedis/onet"
)

// ServiceName is used for registration on the onet.
const ServiceName = "OmniLedger"

// Client is a structure to communicate with the OmniLedger
// service.
// TODO: Fill the structure with the necessary fields for the service.
type Client struct {
	*onet.Client
}

// NewClient instantiates a new OmniLedger client.
// TODO: Write code for NewClient()
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}
