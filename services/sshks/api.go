package sshks

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

/*
 */

// Client is the communication from the SSHksClient to the Cothority
type Client struct {
	*sda.Client
	Config  *SSHksConfig
	Private abstract.Secret
}

// NewClient instantiates a new SSHksConfig client
func NewClient(majority int, cothority *network.Entity) *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

// NewClientFromStream reads the configuration of that client from
// any stream
func NewClientFromStream() (*Client, error)

// SaveClientToStream stores the configuration of the client to a stream
func (c *Client) SaveClientToStream() error {
	return nil
}

// ConfigUpdate asks if there is any new config available that has already
// been approved by others
func (c *Client) ConfigUpdate() error {
	return nil
}

// ConfigNewPropose is sent from a client to the Cothority, signed with its
// private key
func (c *Client) ConfigNewPropose(*SSHksConfig) error {
	return nil
}

// ConfigNewCheck verifies if there is a new configuration awaiting that
// needs approval from clients
func (c *Client) ConfigNewCheck() (*SSHksConfig, error) {
	return nil
}

// ConfigNewVote sends a vote (accept or reject) with regard to a new configuration
func (c *Client) ConfigNewVote(ConfigID, accept bool) error {
	return nil
}
