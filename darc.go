package ocs

import (
	"github.com/dedis/cothority/skipchain"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1/crypto"
)

// DarcID represents one account. It is the concatenation of the skipchain-id
// and a more or less random string.
type DarcID []byte

// Darc stores a new configuration for keys. If the same ID is already
// on the blockchain, it needs to be signed by a threshold of admins in the
// last block. If Admins is nil, no other block with the same ID can be stored.
// If ID is nil, this is a unique block for a single DataOCSWrite.
type Darc struct {
	ID        DarcID
	Accounts  []DarcLink
	Public    []abstract.Point
	Version   int
	Signature *DarcSig
}

func NewDarc(sc skipchain.SkipBlockID) *Darc {
	return &Darc{
		ID: []byte(sc),
	}
}

// DarcLink represents one account with special privileges.
type DarcLink struct {
	ID        DarcID
	Rights    int
	Threshold int
}

const (
	// DarcRightAdmin gives this link the right to add/remove admin
	// accounts.
	DarcRightAdmin = 1 << iota
	// DarcRightLink gives this link the right to add/remove links.
	DarcRightLink
	// DarcRightPublicAdd gives this link the right to add public keys.
	DarcRightPublicAdd
	// DarcRightPublicRemove gives this link the right to remove public keys.
	DarcRightPublicRemove
)

// DarcSig represents a (collective) signature, depending on the signature
// needed to evolve the account-structure.
type DarcSig struct {
	Signers   DarcID
	Version   int
	Signature *crypto.SchnorrSig
}
