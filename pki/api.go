package pki

import (
	"fmt"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
)

// Client is the client to make requests to the PKI service
type Client struct {
	*onet.Client
}

// NewClient makes a new client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// GetProof returns the proofs of possession for the BLS key pairs
func (c *Client) GetProof(si *network.ServerIdentity) ([]PkProof, error) {
	rep := &ResponsePkProof{}
	err := c.SendProtobuf(si, &RequestPkProof{}, rep)
	if err != nil {
		return nil, fmt.Errorf("request failed with: %v", err)
	}

	// make sure the proofs are correct
	for _, srvid := range si.ServiceIdentities {
		if err := rep.Proofs.Verify(&srvid); err != nil {
			return nil, fmt.Errorf("got a wrong proof for service %s: %v", srvid.Name, err)
		}
	}

	return rep.Proofs, nil
}
