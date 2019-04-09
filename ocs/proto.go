package ocs

import (
	"time"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
)

// PROTOSTART
// type :skipchain.SkipBlockID:bytes
// type :time.Time:uint64
// type :byzcoin.Proof:bytes
// type :OCSID:bytes
// package ocs;
// import "onet.proto";
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "OCS";

// ***
// API calls
// ***

// CreateOCS is sent to the service to request a new OCS cothority.
type CreateOCS struct {
	Roster         onet.Roster
	Authentication Auth
}

// CreateOCSReply is the reply sent by the conode if the OCS has been
// setup correctly. It contains the ID of the OCS, which is the binary
// representation of the aggregate public key. It also has the Sig, which
// is the collective signature of all nodes on the aggregate public key
// and the authentication.
type CreateOCSReply struct {
	X   OCSID
	Sig []byte
}

// Reencrypt is sent to the service to request a re-encryption of the
// secret given in Grant. Grant must also contain the proof that the
// request is valid, as well as the ephemeral key, to which the secret
// will be re-encrypted.
type Reencrypt struct {
	X     OCSID
	Grant Grant
}

// ReencryptReply is the reply if the re-encryption is successful, and
// it contains XHat, which is the secret re-encrypted to the ephemeral
// key given in Grant.
type ReencryptReply struct {
	XHat kyber.Point
}

// ***
// Common structures
// ***

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
