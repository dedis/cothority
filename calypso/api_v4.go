package calypso

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
)

// TODO: add LTSID of type kyber.Point
// TODO: think about authentication
// TODO: add CreateAndAuthorise
// TODO: add REST interface

type LTSID kyber.Point

// ClientV4 is a class to communicate to the calypso service.
type ClientV4 struct {
	*onet.Client
}

// NewClientV4 creates a new client to interact with the Calypso Service.
func NewClientV4() *ClientV4 {
	return &ClientV4{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// CreateLTS starts a new Distributed Key Generation with the nodes in the roster and
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
func (c *ClientV4) CreateLTS(ltsRoster *onet.Roster, auth Auth) (X LTSID, sig []byte, err error) {
	return
}

// Reencrypt requests the re-encryption of the secret stored in the grant.
// The grant must also contain the ephemeral key to which the secret will be
// reencrypted to.
// Finally the grant must contain information about how to verify that the
// reencryption request is valid.
//
// This can be called from anywhere.
//
// If the grant is valid, the reencrypted XHat is returned and err is nil. In case
// of error, XHat is nil, and the error will be returned.
func (c *ClientV4) Reencrypt(X kyber.Point, grant Grant) (XHat kyber.Point, err error) {
	return
}
