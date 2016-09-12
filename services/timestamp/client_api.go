package timestamp

import (
	"errors"

	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

// Client is a structure to communicate with the CoSi
// service
type Client struct {
	*sda.Client
}

// NewClient instantiates a new Timestamp client
func NewClient() *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

// SignMsg sends a CoSi sign request
func (c *Client) SignMsg(root *network.ServerIdentity, msg []byte) (*SignatureResponse, error) {
	serviceReq := &SignatureRequest{
		Message: msg,
	}
	log.LLvl4("Sending message [", string(msg), "] to", root)
	reply, err := c.Send(root, serviceReq)
	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}
	sr, ok := reply.Msg.(SignatureResponse)
	if !ok {
		return nil, errors.New("This is odd: couldn't cast reply.")
	}
	return &sr, nil
}

// SetupStamper initializes the root node with the desired configuration
// parameters. The root node will start the main loop upon receiving this
// request.
// XXX This is a quick hack which simplifies the simulations.
func (c *Client) SetupStamper(root *network.ServerIdentity, roster *sda.Roster,
	epochDuration time.Duration) (*SetupRosterResponse, error) {
	serviceReq := &SetupRosterRequest{
		Roster:        roster,
		EpochDuration: epochDuration,
	}
	log.Lvl4("Sending message to:", root)
	reply, err := c.Send(root, serviceReq)
	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}
	sr, ok := reply.Msg.(SetupRosterResponse)
	if !ok {
		return nil, errors.New("This is odd: couldn't cast reply.")
	}
	log.LLvl4("Initialized timestamp with roster id:", sr.ID)
	return &sr, nil
}
