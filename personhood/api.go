package personhood

// api for personhood - very minimalistic for the moment, as most of the
// calls are made from javascript.

import (
	"fmt"

	"go.dedis.ch/cothority/v4"
	"go.dedis.ch/onet/v4"
)

// Client is a structure to communicate with the personhood
// service
type Client struct {
	*onet.Client
}

// NewClient instantiates a new personhood.Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// WipeParties removes all parties stored in the system.
func (c *Client) WipeParties(r onet.Roster) (errs []error) {
	t := true
	pl := PartyList{
		WipeParties: &t,
	}
	for _, si := range r.List {
		err := c.SendProtobuf(si, &pl, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("error in node %s: %s", si.Address, err))
		}
	}
	return
}

// WipeRoPaScis removes all stored RoPaScis from the service.
func (c *Client) WipeRoPaScis(r onet.Roster) (errs []error) {
	t := true
	pl := RoPaSciList{
		Wipe: &t,
	}
	for _, si := range r.List {
		err := c.SendProtobuf(si, &pl, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("error in node %s: %s", si.Address, err))
		}
	}
	return
}
