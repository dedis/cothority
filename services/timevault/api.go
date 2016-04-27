package timevault

import (
	"errors"
	"time"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/timevault"
)

// Client is a structure to communicate with the CoSi
// service
type Client struct {
	*sda.Client
}

// NewClient instantiates a new timevault.Client
func NewClient() *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

// Seal will ask the JVSS group to make a sealing that will be opened when the
// timeout happens.
func (c *Client) Seal(group *sda.EntityList, timeout time.Duration) (*SealResponse, error) {

	if len(group.List) < 1 {
		return nil, errors.New("No members given")
	}
	s := &sealRequest{
		Group:   group,
		Timeout: timeout,
	}

	reply, err := c.Send(group.List[0], s)
	if err != nil {
		return nil, err
	}
	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}

	sr, ok := reply.Msg.(SealResponse)
	if !ok {
		return nil, errors.New("Response invalid...")
	}

	return &sr, nil
}

func (c *Client) Open(group *sda.EntityList, id timevault.SID) (*OpenResponse, error) {
	o := &openRequest{
		Group: group,
		ID:    id,
	}

	reply, err := c.Send(group.List[0], o)
	if err != nil {
		return nil, err
	}
	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}

	resp, ok := reply.Msg.(OpenResponse)
	if !ok {
		return nil, errors.New("Invalid response")
	}

	return &resp, nil
}
