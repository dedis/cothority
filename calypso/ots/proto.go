package ots

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share/pvss"
)

type Write struct {
	Shares   []*pvss.PubVerShare
	Proofs   []kyber.Point
	Publics  []kyber.Point
	CtxtHash []byte
}

type Read struct {
	Write byzcoin.InstanceID
	Xc    kyber.Point
}
