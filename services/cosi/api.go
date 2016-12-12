package service

import (
	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
)

// Client is a structure to communicate with the CoSi
// service
type Client struct {
	*sda.Client
}

// NewClient instantiates a new cosi.Client
func NewClient() *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

// SignatureRequest sends a CoSi sign request to the Cothority defined by the given
// Roster
func (c *Client) SignatureRequest(r *sda.Roster, msg []byte) (*SignatureResponse, error) {
	r2 := &sda.Roster{List: r.List}
	serviceReq := &SignatureRequest{
		Roster:  r2,
		Message: msg,
	}
	if len(r.List) == 0 {
		return nil, errors.New("Got an empty roster-list")
	}
	dst := r.List[0]
	log.Lvl4("Sending message to", dst)
	reply := &SignatureResponse{}
	cerr := c.SendProtobuf(dst, serviceReq, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply, nil
}
