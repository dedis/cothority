package calypso

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
	"time"
)

// TODO: add LTSID of type kyber.Point
// TODO: think about authentication
// TODO: add CreateAndAuthorise
// TODO: add REST interface

// Client is a class to communicate to the calypso service.
type Client struct {
	*onet.Client
}

// NewClient creates a new client to interact with the Calypso Service.
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// CreateLTS starts a new Distributed Key Generation with the nodes in the roster and
// return the collective public key X. This X is also used later to identify the
// LTS instance, as there can be more than one LTS group on a node.
//
// This can only be called from localhost, except if the environment variable
// COTHORITY_ALLOW_INSECURE_ADMIN is set to 'true'.
//
// In case of error, X is nil, and the error indicates what is wrong.
//
// TODO: return the DKG proof instead of the aggregate public key
func (c *Client) CreateLTS(ltsRoster *onet.Roster) (X kyber.Point, err error) {
	return
}

// Authorise sets up an authorisation option for this node. All the nodes that
// are part of the LTS need to be set up with the same authorisation option.
//
// As with CreateLTS, this API can only be called from localhost, except if
// the environment variable COTHORITY_ALLOW_INSECURE_ADMIN is set to 'true'.
//
// If the authorisation is stored successfully, nil is returned.
func (c *Client) Authorise(X kyber.Point, auth Auth) (err error) {
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
func (c *Client) Reencrypt(X kyber.Point, grant Grant) (XHat kyber.Point, err error) {
	return
}

// Auth holds all possible authentication structures. When using it to call
// Authorise, only one of the fields must be non-nil.
type Auth struct {
	ByzCoin           *AuthByzCoin
	// TODO: rename to X509Cert
	HyperLedgerFabric *AuthX509Cert
	Ethereum          *AuthEthereum
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

// AuthEthereum holds the information necessary to authenticate an Ethereum
// request. It holds the ContractAddress that serves as the basis to generate
// new requests.
type AuthEthereum struct {
	ContractAddress []byte
}

// Grant holds one of the possible grant proofs for a reencryption request. Each
// grant proof must hold the secret to be reencrypted, the ephemeral key, as well
// as the proof itself that the request is valid. For each of the authentication
// schemes, this proof will be different.
type Grant struct {
	ByzCoin  *GrantByzCoin
	X509Cert *GrantX509Cert
	Ethereum *GrantEthereum
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

// GrantEthereum holds the proof that the read request has been successfully stored
// in Ethereum.
type GrantEthereum struct {
	Contract []byte
}
