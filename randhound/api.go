package randhound

import "gopkg.in/dedis/onet.v1"

// Client is a Pulsar[RandHound] client that can communicate with the corresponding service.
type Client struct {
	*onet.Client
}

// NewClient constructor of Pulsar[RandHound] clients.
func NewClient() *Client {
	return &Client{Client: onet.NewClient("Pulsar[RandHound]")}
}

// Setup sends a message to a node of the given roster (currently the one at
// index 0) to request the setup of a Pulsar[RandHound] service.
func (c *Client) Setup(roster *onet.Roster, groups int, purpose string,
	interval int) (*SetupReply, onet.ClientError) {
	dst := roster.List[0]
	request := &SetupRequest{
		Roster:   roster,
		Groups:   groups,
		Purpose:  purpose,
		Interval: interval,
	}
	reply := &SetupReply{}
	if err := c.SendProtobuf(dst, request, reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// Random allows to request collective randomness from a running
// Pulsar[RandHound] service.
func (c *Client) Random(roster *onet.Roster) (*RandReply, onet.ClientError) {
	dst := roster.List[0]
	request := &RandRequest{}
	reply := &RandReply{}
	if err := c.SendProtobuf(dst, request, reply); err != nil {
		return nil, err
	}
	return reply, nil
}
