package authentication

import (
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/auth/darc"
	"github.com/dedis/onet"
)

// Client is used to interact with the authentication service from a
// go cli.
type Client struct {
	*onet.Client
}

// NewClient returns a client that points to the Authentication service.
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// GetPolicy returns the latest version of the chosen policy with the ID
// given. If the ID == null, the latest version of the
// basic policy is returned.
func (c *Client) GetPolicy(ID darc.ID) (*darc.Darc, error) {
	return nil, errors.New("not yet implemented")
}

//
// UpdatePolicy updates an existing policy. Following the policy-library,
// it needs to be signed by one of the admin users.
func (c *Client) UpdatePolicy(newPolicy darc.Darc) error {
	return errors.New("not yet implemented")
}

// UpdatePolicyPIN can be used in case the private key is not available,
// but if the user has access to the logs of the server. On the first
// call the PIN == "", and the server will print a 6-digit PIN in the log
// files. When he receives the policy and the correct PIN, the server will
// auto-sign the policy using his private key and add it to the policy-list.
func (c *Client) UpdatePolicyPIN(newPolicy darc.Darc, PIN string) error {
	return errors.New("not yet implemented")
}

// AddPolicy can be used to add a new policy to the system that will be
// referenced later with UpdatePolicy. For a new policy, it must be signed
// by a user of the root-policy.
func (c *Client) AddPolicy(newPolicy darc.Darc, signature darc.Signature) error {
	return errors.New("not yet implemented")
}
