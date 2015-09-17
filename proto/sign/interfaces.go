package sign

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/helpers/hashid"
	"github.com/dedis/cothority/helpers/proof"
)

var DEBUG bool // to avoid verifying paths and signatures all the time

// Returns commitment contribution for a round
type CommitFunc func(view int) []byte

// Called at the end of a round
// Allows client of Signer to receive signature, proof, and error via RPC
type DoneFunc func(view int, SNRoot hashid.HashId, LogHash hashid.HashId, p proof.Proof)

// todo: see where Signer should be located
type Signer interface {
	Name() string
	IsRoot(view int) bool
	Suite() abstract.Suite
	StartSigningRound() error
	StartVotingRound(v *Vote) error

	LastRound() int       // last round number seen by Signer
	SetLastSeenRound(int) // impose change in round numbering

	Hostlist() []string

	// // proof can be nil for simple non Merkle Tree signatures
	// // could add option field for Sign
	// Sign([]byte) (hashid.HashId, proof.Proof, error)

	// registers a commitment function to be called
	// at the start of every round
	RegisterAnnounceFunc(cf CommitFunc)

	RegisterDoneFunc(df DoneFunc)

	// Allows user of Signer to inform Signer to run with simulated failures
	// As to test robustness of Signer
	SetFailureRate(val int)

	ViewChangeCh() chan string

	Close()
	Listen() error

	AddSelf(host string) error
	RemoveSelf() error
}
