package ots

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/calypso/ots/protocol"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share/pvss"
	"go.dedis.ch/onet/v3"
)

type Write struct {
	PolicyID darc.ID
	Shares   []*pvss.PubVerShare
	Proofs   []kyber.Point
	Publics  []kyber.Point
	CtxtHash []byte
}

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
