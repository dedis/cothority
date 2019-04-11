package ocs

import (
	"time"

	"go.dedis.ch/cothority/v3/darc"

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
// It holds the two policies necessary to define an OCS: how to
// authenticate a reencryption request, and how to authenticate a
// resharing request.
// In the current form, both policies point to the same structure. If at
// a later moment a new access control backend is added, it might be that
// the policies will differ for this new backend.
type CreateOCS struct {
	Roster          onet.Roster
	PolicyReencrypt Policy
	PolicyReshare   Policy
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
// secret given in AuthReencrypt. AuthReencrypt must also contain the proof that the
// request is valid, as well as the ephemeral key, to which the secret
// will be re-encrypted.
type Reencrypt struct {
	X    OCSID
	Auth AuthReencrypt
}

// MessageReencryptReply is the reply if the re-encryption is successful, and
// it contains XHat, which is the secret re-encrypted to the ephemeral
// key given in AuthReencrypt.
type ReencryptReply struct {
	X       kyber.Point
	XhatEnc kyber.Point
	C       kyber.Point
}

// Reshare is called to ask OCS to change the roster. It needs a valid
// authentication before the private keys are re-distributed over the new
// roster.
// TODO: should NewRoster be always present in AuthReshare? It will be present
// TODO: at least in AuthReshareByzCoin, but might not in other AuthReshares
type Reshare struct {
	X         OCSID
	NewRoster onet.Roster
	Auth      AuthReshare
}

// ReshareReply is returned if the resharing has been completed successfully
// and contains the collective signature on the message
//   sha256( X | NewRoster )
type ReshareReply struct {
	Sig []byte
}

// ***
// Common structures
// ***

// Policy holds all possible authentication structures. When using it to call
// Authorise, only one of the fields must be non-nil.
type Policy struct {
	ByzCoin  *PolicyByzCoin
	X509Cert *PolicyX509Cert
}

// PolicyByzCoin holds the information necessary to authenticate a byzcoin request.
// In the ByzCoin model, all requests are valid as long as they are stored in the
// blockchain with the given ID.
// The TTL is to avoid that too old requests are re-used. If it is 0, it is disabled.
type PolicyByzCoin struct {
	ByzCoinID skipchain.SkipBlockID
	TTL       time.Time
}

// X509Cert holds the information necessary to authenticate a HyperLedger/Fabric
// request. In its simplest form, it is simply the CA that will have to sign the
// certificates of the requesters.
// The Threshold indicates how many clients must have signed the request before it
// is accepted.
type PolicyX509Cert struct {
	// Slice of ASN.1 encoded X509 certificates.
	CA        [][]byte
	Threshold int
}

// AuthReencrypt holds one of the possible authentication proofs for a reencryption request. Each
// authentication proof must hold the secret to be reencrypted, the ephemeral key, as well
// as the proof itself that the request is valid. For each of the authentication
// schemes, this proof will be different.
type AuthReencrypt struct {
	Ephemeral kyber.Point
	ByzCoin   *AuthReencryptByzCoin
	X509Cert  *AuthReencryptX509Cert
}

// AuthReencryptByzCoin holds the proof of the write instance, holding the secret itself.
// The proof of the read instance holds the ephemeral key. Both proofs can be
// verified using one of the stored ByzCoinIDs.
type AuthReencryptByzCoin struct {
	// Write is the proof containing the write request.
	Write byzcoin.Proof
	// Read is the proof that he has been accepted to read the secret.
	Read byzcoin.Proof
	// Ephemeral can be non-nil to point to a key to which the data needs to be
	// re-encrypted to, but then Signature also needs to be non-nil.
	Ephemeral kyber.Point
	// If Ephemeral si non-nil, it must be signed by the darc responsible for the
	// Read instance to make sure it's a valid reencryption-request.
	Signature *darc.Signature
}

// AuthReencryptX509Cert holds the proof that at least a threshold number of clients
// accepted the reencryption.
// For each client, there must exist a certificate that can be verified by the
// CA certificate from X509Cert. Additionally, each client must sign the
// following message:
//   sha256( Secret | Ephemeral | Time )
type AuthReencryptX509Cert struct {
	U            kyber.Point
	Certificates [][]byte
}

// AuthReshare holds the proof that at least a threshold number of clients accepted the
// request to reshare the secret key. The authentication must hold the new roster, as
// well as the proof that the new roster should be applied to a given OCS.
type AuthReshare struct {
	ByzCoin  *AuthReshareByzCoin
	X509Cert *AuthReshareX509Cert
}

// AuthReshareByzCoin holds the byzcoin-proof that contains the latest OCS-instance
// which includes the roster. The OCS-nodes will make sure that the version of the
// OCS-instance is bigger than the current version.
type AuthReshareByzCoin struct {
	Reshare byzcoin.Proof
}

// AuthReshareX509Cert holds the X509 proof that the new roster is valid.
type AuthReshareX509Cert struct {
	Certificates [][]byte
}
