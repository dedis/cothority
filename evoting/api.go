// Package evoting is client side API for communicating with the evoting
// service.
package evoting

import (
	"go.dedis.ch/cothority/v3/skipchain"
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
	Roster    *onet.Roster
}

// NewClient instantiates a new evoting.Client.
func NewClient(roster *onet.Roster) *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName), Roster: roster}
}

// Ping a random server which increments the nonce.
func (c *Client) Ping(nonce uint32) (*Ping, error) {
	dest := c.Roster.RandomServerIdentity()
	reply := &Ping{}
	if err := c.SendProtobuf(dest, &Ping{Nonce: nonce}, reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// LookupSciper returns information about a sciper number.
func (c *Client) LookupSciper(sciper string) (reply *LookupSciperReply, err error) {
	reply = &LookupSciperReply{}
	err = c.SendProtobuf(c.Roster.RandomServerIdentity(), &LookupSciper{Sciper: sciper, LookupURL: c.LookupURL}, reply)
	return
}

// Reconstruct returns the reconstructed votes.
// If the election is not yet finished, it will return an error
func (c *Client) Reconstruct(id skipchain.SkipBlockID) (rr ReconstructReply, err error) {
	err = c.SendProtobuf(c.Roster.List[0], &Reconstruct{
		ID: id,
	}, &rr)
	return
}
