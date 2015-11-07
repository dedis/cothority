package sign

import (
	"github.com/dedis/crypto/abstract"
)

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

	// Installs new callbacks for the steps of the signing-algorithm
	SetCallbacks(Callbacks)

	// Allows user of Signer to inform Signer to run with simulated failures
	// As to test robustness of Signer
	SetFailureRate(val int)

	ViewChangeCh() chan string

	Close()
	CloseAll(int) error
	GetView() int
	Listen() error

	AddSelf(host string) error
	RemoveSelf() error
}
