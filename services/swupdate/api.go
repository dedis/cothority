package swupdate

import "github.com/dedis/cothority/sda"

// Client is a structure to communicate with the software-update service.
type Client struct {
	*sda.Client
	Policy *Policy
	Roster *sda.Roster
	ProjectID
}

// NewClient instantiates a new communication with the swupdate-client.
func NewClient(r *sda.Roster, policy *Policy) *Client {
	return &Client{
		Client: sda.NewClient(ServiceName),
		Roster: r,
	}
}
