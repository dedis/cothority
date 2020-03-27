package calypso

import (
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
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

// TODO: Ceyhun
type SemiWrite struct {
	//Data     []byte
	DataHash  []byte
	K         kyber.Point
	C         kyber.Point
	Reader    kyber.Point
	EncReader []byte
}

// Write is the data stored in a write instance. It stores a reference to the LTS
// used and the encrypted secret.
type Write struct {
	// Data should be encrypted by the application under the symmetric key in U and Cs
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
	// f is the proof - written in uppercase here so it is an exported field,
	// but in the OCS-paper it's lowercase.
	F kyber.Scalar
	// Cs are the ElGamal parts for the symmetric key material (might
	// also contain an IV)
	Cs []kyber.Point
	// ExtraData is clear text and application-specific
	ExtraData []byte `protobuf:"opt"`
	// LTSID points to the identity of the lts group
	LTSID []byte
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

// CreateLTS is used to start a DKG and store the private keys in each node.
type CreateLTS struct {
	// Roster is the list of nodes that should participate in the DKG.
	Roster onet.Roster
	// BCID is the ID of the ByzCoin ledger that can use this LTS.
	BCID skipchain.SkipBlockID
}

// CreateLTSReply is returned upon successfully setting up the distributed
// key.
type CreateLTSReply struct {
	// LTSID is a random 32-byte slice that represents the LTS.
	LTSID []byte
	// X is the public key of the LTS.
	X kyber.Point
	// TODO: can we remove the LTSID and only use the public key to identify
	// an LTS?
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
	// Cs are the secrets re-encrypted under the reader's public key.
	Cs []kyber.Point
	// XhatEnc is the random part of the encryption.
	XhatEnc kyber.Point
	// X is the aggregate public key of the LTS used.
	X kyber.Point
}

// SharedPublic asks for the shared public key of the corresponding LTSID
type SharedPublic struct {
	// LTSID is the id of the LTS instance created.
	LTSID []byte
}

// SharedPublicReply sends back the shared public key.
type SharedPublicReply struct {
	// X is the distributed public key.
	X kyber.Point
}
