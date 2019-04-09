package calypso

import (
	"time"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
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

//
// V4 proposed extensions
//

// Auth holds all possible authentication structures. When using it to call
// Authorise, only one of the fields must be non-nil.
type Auth struct {
	ByzCoin      *AuthByzCoin
	AuthX509Cert *AuthX509Cert
}

// AuthByzCoin holds the information necessary to authenticate a byzcoin request.
// In the ByzCoin model, all requests are valid as long as they are stored in the
// blockchain with the given ID.
// The TTL is to avoid that too old requests are re-used. If it is 0, it is disabled.
type AuthByzCoin struct {
	ByzCoinID skipchain.SkipBlockID
	TTL       time.Time
}

// AuthX509Cert holds the information necessary to authenticate a HyperLedger/Fabric
// request. In its simplest form, it is simply the CA that will have to sign the
// certificates of the requesters.
// The Threshold indicates how many clients must have signed the request before it
// is accepted.
type AuthX509Cert struct {
	// Slice of ASN.1 encoded X509 certificates.
	CA        [][]byte
	Threshold int
}

// Grant holds one of the possible grant proofs for a reencryption request. Each
// grant proof must hold the secret to be reencrypted, the ephemeral key, as well
// as the proof itself that the request is valid. For each of the authentication
// schemes, this proof will be different.
type Grant struct {
	ByzCoin  *GrantByzCoin
	X509Cert *GrantX509Cert
}

// GrantByzCoin holds the proof of the write instance, holding the secret itself.
// The proof of the read instance holds the ephemeral key. Both proofs can be
// verified using one of the stored ByzCoinIDs.
type GrantByzCoin struct {
	// Write is the proof containing the write request.
	Write byzcoin.Proof
	// Read is the proof that he has been accepted to read the secret.
	Read byzcoin.Proof
}

// GrantX509Cert holds the proof that at least a threshold number of clients
// accepted the reencryption.
// For each client, there must exist a certificate that can be verified by the
// CA certificate from AuthX509Cert. Additionally, each client must sign the
// following message:
//   sha256( Secret | Ephemeral | Time )
type GrantX509Cert struct {
	Secret       kyber.Point
	Certificates [][]byte
}
