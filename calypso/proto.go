package calypso

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
// package calypso;
// import "byzcoin.proto";
// import "onet.proto";
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "Calypso";

// ***
// Common structures
// ***

// Write is the data stored in a write instance. It stores a reference to the LTS
// used and the encrypted secret.
type Write struct {
	// Data should be encrypted by the application under the symmetric key
	// in U and C
	Data []byte
	// U is the encrypted random value for the ElGamal encryption
	U kyber.Point
	// Ubar, E and f will be used by the server to verify the writer did
	// correctly encrypt the key. It binds the policy (the darc) with the
	// cyphertext.
	// Ubar is used for the log-equality proof
	Ubar kyber.Point
	// E is the non-interactive challenge as scalar
	E kyber.Scalar
	// f is the proof - written in uppercase here so it is an exported
	// field, but in the OCS-paper it's lowercase.
	F kyber.Scalar
	// C is the ElGamal parts for the symmetric key material (might also
	// contain an IV)
	C kyber.Point
	// ExtraData is clear text and application-specific
	ExtraData []byte `protobuf:"opt"`
	// LTSID points to the identity of the lts group
	LTSID byzcoin.InstanceID
}

// Read is the data stored in a read instance. It has a pointer to the write
// instance and the public key used to create the read instance.
type Read struct {
	Write byzcoin.InstanceID
	Xc    kyber.Point
}

// ***
// These are the messages used in the API-calls
// ***

// Authorise is used to add the given ByzCoinID into the list of
// authorised IDs.
type Authorise struct {
	ByzCoinID skipchain.SkipBlockID
}

// AuthoriseReply is returned upon successful authorisation.
type AuthoriseReply struct {
}

// CreateLTS is used to start a DKG and store the private keys in each node.
// Prior to using this request, the Calypso roster must be recorded on the
// ByzCoin blockchain in the instance specified by InstanceID.
type CreateLTS struct {
	Proof byzcoin.Proof
}

// CreateLTSReply is returned upon successfully setting up the distributed
// key.
type CreateLTSReply struct {
	ByzCoinID  skipchain.SkipBlockID
	InstanceID byzcoin.InstanceID
	// X is the public key of the LTS.
	X kyber.Point
}

// ReshareLTS is used to update the LTS shares. Prior to using this request,
// the Calypso roster must be updated on the ByzCoin blockchain in the instance
// specified by InstanceID.
type ReshareLTS struct {
	Proof byzcoin.Proof
}

// ReshareLTSReply is returned upon successful resharing. The LTSID and the
// public key X should remain the same.
type ReshareLTSReply struct {
}

// DecryptKey is sent by a reader after he successfully stored a 'Read' request
// in byzcoin Client.
type DecryptKey struct {
	// Read is the proof that he has been accepted to read the secret.
	Read byzcoin.Proof
	// Write is the proof containing the write request.
	Write byzcoin.Proof
}

// DecryptKeyReply is returned if the service verified successfully that the
// decryption request is valid.
type DecryptKeyReply struct {
	// C is the secret re-encrypted under the reader's public key.
	C kyber.Point
	// XhatEnc is the random part of the encryption.
	XhatEnc kyber.Point
	// X is the aggregate public key of the LTS used.
	X kyber.Point
}

// GetLTSReply asks for the shared public key of the corresponding LTSID
type GetLTSReply struct {
	// LTSID is the id of the LTS instance created.
	LTSID byzcoin.InstanceID
}

// LtsInstanceInfo is the information stored in an LTS instance.
type LtsInstanceInfo struct {
	Roster onet.Roster
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
