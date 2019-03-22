package calypso

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
	"time"
)

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
	HyperLedgerFabric *AuthHLFabric
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

// AuthHLFabric holds the information necessary to authenticate a HyperLedger/Fabric
// request. In its simplest form, it is simply the CA that will have to sign the
// certificates of the requesters.
// The TTL is to avoid that too old requests are re-used. If it is 0, it is disabled.
// The Threshold indicates how many clients must have signed the request before it
// is accepted.
type AuthHLFabric struct {
	CA        []byte
	TTL       time.Time
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
	HLFabric *GrantHLFabric
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

// GrantHLFabric holds the proof that at least a threshold number of clients
// accepted the reencryption.
// For each client, there must exist a certificate that can be verified by the
// CA certificate from AuthHLFabric. Additionally, each client must sign the
// following message:
//   sha256( Secret | Ephemeral | Time )
type GrantHLFabric struct {
	Secret       kyber.Point
	Ephemeral    kyber.Point
	Time         time.Time
	Certificates [][]byte
	Signatures   [][]byte
}

// GrantEthereum holds the proof that the read request has been successfully stored
// in Ethereum.
type GrantEthereum struct {
	Contract []byte
}
