package sign

import (
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
)

var DEBUG bool // to avoid verifying paths and signatures all the time

// Func to call when we receive a announcement
type AnnounceFunc func(msg *AnnouncementMessage)

// Returns commitment contribution for a round
type CommitFunc func(view int) []byte

// Called at the end of a round
// Allows client of Signer to receive signature, proof, and error
type DoneFunc func(view int, SNRoot hashid.HashId, LogHash hashid.HashId, p proof.Proof,
	signature *SignatureBroadcastMessage, suite abstract.Suite)

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

	// Registers a announcement function that is to be called for every
	// Announcement message a node receives (the message can contains some useful data .. ;)
	RegisterAnnounceFunc(af AnnounceFunc)
	// registers a commitment function to be called
	// for every commit phase. It returns the aggregated merkle root for a node
	RegisterCommitFunc(cf CommitFunc)

	RegisterDoneFunc(df DoneFunc)

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
