package protocol

import (
	"github.com/dedis/onet"
	"github.com/dedis/cothority/evoting/lib"
	"sync"
	"github.com/dedis/cothority/skipchain"
)

// PROTOSTART

// Decrypt is the core structure of the protocol.
type Decrypt struct {
	*onet.TreeNodeInstance

	User      uint32
	Signature []byte

	Secret   *lib.SharedSecret
	// Secret is the private key share from the DKG.
	Election *lib.Election
	// Election to be decrypted.

	Finished           chan bool
	// Flag to signal protocol termination.
	LeaderParticipates bool
	// LeaderParticipates is a flag to denote if leader should calculate the partial.
	successReplies     int
	mutex              sync.Mutex

	Skipchain *skipchain.Service
}

// Shuffle is the core structure of the protocol.
type Shuffle struct {
	*onet.TreeNodeInstance

	User      uint32
	Signature []byte
	Election  *lib.Election
	// Election to be shuffled.

	Finished chan error
	// Flag to signal protocol termination.

	Skipchain          *skipchain.Service
	LeaderParticipates bool
	// LeaderParticipates is a flag that denotes if leader should participate in the shuffle
}


