package randhound

import "gopkg.in/dedis/onet.v1"

// Client ...
type Client struct {
	*onet.Client
}

// NewClient ...
func NewClient() *Client {
	return &Client{Client: onet.NewClient("RandHound")}
}

// Setup ...
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

// Random ...
func (c *Client) Random(roster *onet.Roster) (*RandReply, onet.ClientError) {
	dst := roster.List[0]
	request := &RandRequest{}
	reply := &RandReply{}
	if err := c.SendProtobuf(dst, request, reply); err != nil {
		return nil, err
	}
	return reply, nil
}
