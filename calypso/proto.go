package calypso

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
)

// PROTOSTART
// type :skipchain.SkipBlockID:bytes
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
	// Cost reflects how many coins you'll have to pay for a read-request
	Cost byzcoin.Coin `protobuf:"opt"`
}

// Read is the data stored in a read instance. It has a pointer to the write
// instance and the public key used to re-encrypt the secret to.
type Read struct {
	Write byzcoin.InstanceID
	Xc    kyber.Point
}

// ***
// These are the messages used in the API-calls
// ***

// Authorise adds a ByzCoinID to the list of authorized IDs.
// Deprecated: please use Authorize.
type Authorise struct {
	ByzCoinID skipchain.SkipBlockID
}

// AuthoriseReply is returned upon successful authorization
// Deprecated: please use AuthorizeReply
type AuthoriseReply struct {
}

// Authorize is used to add the given ByzCoinID into the list of
// authorised IDs. To be accepted, the ByzCoinID must be signed
// using the private key of the conode.
type Authorize struct {
	ByzCoinID skipchain.SkipBlockID
	Timestamp int64  `protobuf:"opt"`
	Signature []byte `protobuf:"opt"`
}

// AuthorizeReply is returned upon successful authorisation.
type AuthorizeReply struct {
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
