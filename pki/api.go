package pki

import (
	"bytes"
	"errors"

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
func (c *Client) GetProof(srvid *network.ServerIdentity) ([]PkProof, error) {
	nonce, err := makeNonce()
	if err != nil {
		return nil, err
	}

	req := &RequestPkProof{Nonce: nonce}
	rep := &ResponsePkProof{}

	err = c.SendProtobuf(srvid, req, rep)
	if err != nil {
		return nil, err
	}

	// make sure correct nonce has been used
	for _, proof := range rep.Proofs {
		if !bytes.Equal(proof.Nonce[:nonceLength], nonce) {
			return nil, errors.New("nonce does not match with the request")
		}
	}

	return rep.Proofs, nil
}
