package pq

import "go.dedis.ch/kyber/v3/share"

type VerifyWrite struct {
	Idx   int
	Write *Write
	Share *share.PriShare
	Rand  []byte
}

type VerifyWriteReply struct {
	Sig []byte
}
