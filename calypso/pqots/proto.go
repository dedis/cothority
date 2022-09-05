package pqots

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/calypso/pqots/protocol"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/onet/v3"
)

type Write struct {
	CtxtHash    []byte
	Publics     []kyber.Point
	Commitments [][]byte
}

type WriteTxn struct {
	Threshold int
	Write     Write
	Sigs      map[int][]byte
}

type VerifyWriteRequest struct {
	Idx   int
	Write *Write
	Share *share.PriShare
	Rand  []byte
}

type VerifyWriteReply struct {
	Sig []byte
}

// Read is the data stored in a read instance. It has a pointer to the write
// instance and the public key used to re-encrypt the secret to.
type Read struct {
	Write byzcoin.InstanceID
	Xc    kyber.Point
}

type DecryptKeyRequest struct {
	Roster *onet.Roster
	// Read is the proof that he has been accepted to read the secret.
	Read byzcoin.Proof
	// Write is the proof containing the write request.
	Write byzcoin.Proof
}

type DecryptKeyReply struct {
	Reencryptions []*protocol.EGP
}
