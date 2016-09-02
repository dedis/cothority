package byzcoin_ng

/*
The api.go defines the methods that can be called from the outside. Most
of the methods will take a roster so that the service knows which nodes
it should work with.

This part of the service runs on the client or the app.
*/

import (
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
