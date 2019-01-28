// Package evoting is client side API for communicating with the evoting
// service.
package evoting

import (
	"go.dedis.ch/onet/v3"

	"go.dedis.ch/cothority/v3"
)

// ServiceName is the identifier of the service (application name).
const ServiceName = "evoting"

// Client is a structure to communicate with the evoting service.
type Client struct {
	*onet.Client
	// If LookupURL is set, use it for SCIPER lookups (for tests).
	LookupURL string
}

// NewClient instantiates a new evoting.Client.
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// Ping a random server which increments the nonce.
func (c *Client) Ping(roster *onet.Roster, nonce uint32) (*Ping, error) {
	dest := roster.RandomServerIdentity()
	reply := &Ping{}
	if err := c.SendProtobuf(dest, &Ping{Nonce: nonce}, reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// LookupSciper returns information about a sciper number.
func (c *Client) LookupSciper(roster *onet.Roster, sciper string) (reply *LookupSciperReply, err error) {
	reply = &LookupSciperReply{}
	err = c.SendProtobuf(roster.RandomServerIdentity(), &LookupSciper{Sciper: sciper, LookupURL: c.LookupURL}, reply)
	return
}
