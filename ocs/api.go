package ocs

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
)

// TODO: think about authentication
// TODO: add REST interface

type OCSID kyber.Point

// Client is a class to communicate to the calypso service.
type Client struct {
	*onet.Client
}

// NewClientV4 creates a new client to interact with the Calypso Service.
func NewClientV4() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// CreateOCS starts a new Distributed Key Generation with the nodes in the roster and
// returns the collective public key X. This X is also used later to identify the
// LTS instance, as there can be more than one LTS group on a node.
//
// It also sets up an authorisation option for the nodes.
//
// This can only be called from localhost, except if the environment variable
// COTHORITY_ALLOW_INSECURE_ADMIN is set to 'true'.
//
// In case of error, X is nil, and the error indicates what is wrong.
// The `sig` returned is a collective signature on the following hash:
//   sha256( X | protobuf.Encode(auth) )
// It can be verified using the aggregate service key from the roster:
//   msg := sha256.New()
//   Xbuf, err := X.MarshalBinary()
//   // Check for errors
//   msg.Write(Xbuf)
//   authBuf, err := protobuf.Encode(auth)
//   // Check for errors
//   err = schnorr.Verify(cothority.Suite, roster.ServiceAggregate(calypso.ServiceName),
//       msg.Sum(nil), sig)
//   // If err == nil, the signature is correct
func (c *Client) CreateOCS(roster onet.Roster, policyReencrypt, policyReshare Policy) (X OCSID, sig []byte, err error) {
	var ret CreateOCSReply
	err = c.SendProtobuf(roster.RandomServerIdentity(), &CreateOCS{
		Roster:          roster,
		PolicyReencrypt: policyReencrypt,
		PolicyReshare:   policyReshare,
	}, &ret)
	if err != nil {
		return
	}
	return ret.X, ret.Sig, nil
}

// Reencrypt requests the re-encryption of the secret stored in the authentication.
// The authentication must also contain the ephemeral key to which the secret will be
// reencrypted to.
// Finally the authentication must contain information about how to verify that the
// reencryption request is valid.
//
// This can be called from anywhere.
//
// If the authentication is valid, the reencrypted XHat is returned and err is nil. In case
// of error, XHat is nil, and the error will be returned.
func (c *Client) Reencrypt(roster onet.Roster, X OCSID, auth AuthReencrypt) (XHat kyber.Point, err error) {
	var ret ReencryptReply
	err = c.SendProtobuf(roster.RandomServerIdentity(), &Reencrypt{X: X, Auth: auth}, &ret)
	if err != nil {
		return
	}
	return ret.X, nil
}

// Reshare requests the OCS X to share the private key to a new set of nodes given in newRoster.
// The auth argument must give proof that this request is valid.
//
// If the request was successful, nil is returned, an error otherwise.
func (c *Client) Reshare(oldRoster onet.Roster, X OCSID, newRoster onet.Roster, auth AuthReshare) error {
	return c.SendProtobuf(oldRoster.RandomServerIdentity(), &Reshare{X: X, NewRoster: newRoster, Auth: auth}, nil)
}
