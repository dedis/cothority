package ocs

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
)

// TODO: think about authentication
// TODO: add REST interface

type OCSID []byte

// Client is a class to communicate to the calypso service.
type Client struct {
	*onet.Client
}

// NewClient creates a new client to interact with the Calypso Service.
func NewClient() *Client {
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
func (c *Client) CreateOCS(roster onet.Roster, policyReencrypt, policyReshare Policy) (OcsID OCSID, err error) {
	var ret CreateOCSReply
	err = c.SendProtobuf(roster.List[0], &CreateOCS{
		Roster:          roster,
		PolicyReencrypt: policyReencrypt,
		PolicyReshare:   policyReshare,
	}, &ret)
	if err != nil {
		return
	}
	return ret.OcsID, nil
}

// GetProofs calls all nodes in turn to get their view of the OCS given in the call. The
// returned OCSProof contains all necessary material to convince an outside client that
// the OCS is correctly set up. The client should be careful to verify that the returned
// policies match the policies he knows to be good.
func (c *Client) GetProofs(roster onet.Roster, OcsID OCSID) (op OCSProof, err error) {
	for _, si := range roster.List {
		var reply GetProofReply
		err = c.SendProtobuf(si, &GetProof{OcsID}, &reply)
		if err != nil {
			err = Erret(err)
			return
		}
		if len(op.Signatures) == 0 {
			op = reply.Proof
		} else {
			op.Signatures = append(op.Signatures, reply.Proof.Signatures[0])
		}
	}
	return op, op.Verify()
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
func (c *Client) Reencrypt(roster onet.Roster, OcsID OCSID, auth AuthReencrypt) (XHatEnc kyber.Point, err error) {
	var ret ReencryptReply
	err = c.SendProtobuf(roster.RandomServerIdentity(), &Reencrypt{OcsID: OcsID, Auth: auth}, &ret)
	if err != nil {
		return
	}
	return ret.XhatEnc, nil
}

// Reshare requests the OCS X to share the private key to a new set of nodes given in newRoster.
// The auth argument must give proof that this request is valid.
//
// If the request was successful, nil is returned, an error otherwise.
func (c *Client) Reshare(oldRoster onet.Roster, X OCSID, newRoster onet.Roster, auth AuthReshare) error {
	return c.SendProtobuf(oldRoster.RandomServerIdentity(), &Reshare{OcsID: X, NewRoster: newRoster, Auth: auth}, nil)
}
