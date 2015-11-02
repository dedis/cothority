package sign

import (
	"bytes"
	"encoding/binary"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
	"time"
)

var DEBUG bool // to avoid verifying paths and signatures all the time

// Func to call to get the message that will bebroadcasted down the tree for
// each new round
type RoundMessageFunc func(nRound int) []byte

// DefaultRoundMessageFunc returns the current timestamp
func DefaultRoundMessageFunc(nRound int) []byte {
	ts := time.Now().UTC()
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, ts.Unix())
	return b.Bytes()
}

// Func to call when we receive a announcement
type OnAnnounceFunc func(msg *AnnouncementMessage)

// Returns commitment contribution for a round
type CommitFunc func(view int) []byte

// Called at the end of a round
// Allows client of Signer to receive signature, proof, and error
type OnDoneFunc func(view int, SNRoot hashid.HashId, LogHash hashid.HashId, p proof.Proof,
	signature *SignatureBroadcastMessage)

// Validation-mode validate what's being signed for this round
type ValidateFunc func(vbm *ValidationBroadcastMessage) bool

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
	// RegisterRoundMessage is the method to call when generating th emessage
	// for a new round
	RegisterRoundMessageFunc(rm RoundMessageFunc)
	// Registers a announcement function that is to be called for every
	// Announcement message a node receives (the message can contains some useful data .. ;)
	RegisterOnAnnounceFunc(af OnAnnounceFunc)
	// registers a commitment function to be called
	// for every commit phase. It returns the aggregated merkle root for a node
	RegisterCommitFunc(cf CommitFunc)

	RegisterOnDoneFunc(df OnDoneFunc)
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
