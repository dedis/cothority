package lib

import (
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/kyber"
	"github.com/dedis/onet/network"
)

// PROTOSTART

// Election is the base object for a voting procedure. It is stored
// in the second skipblock right after the (empty) genesis block. A reference
// to the election skipchain is appended to the master skipchain upon opening.
type Election struct {
	Name    map[string]string
	// Name of the election. lang-code, value pair
	Creator uint32
	// Creator is the election responsible.
	Users   []uint32
	// Users is the list of registered voters.

	ID        skipchain.SkipBlockID
	// ID is the hash of the genesis block.
	Master    skipchain.SkipBlockID
	// Master is the hash of the master skipchain.
	Roster    *onet.Roster
	// Roster is the set of responsible nodes.
	Key       kyber.Point
	// Key is the DKG public key.
	MasterKey kyber.Point
	// MasterKey is the front-end public key.
	Stage     ElectionState
	// Stage indicates the phase of election and is used for filtering in frontend

	Candidates []uint32
	// Candidates is the list of candidate scipers.
	MaxChoices int
	// MaxChoices is the max votes in allowed in a ballot.
	Subtitle   map[string]string
	// Description in string format. lang-code, value pair
	MoreInfo   string
	// MoreInfo is the url to AE Website for the given election.
	Start      int64
	// Start denotes the election start unix timestamp
	End        int64
	// End (termination) datetime as unix timestamp.

	Theme  string
	// Theme denotes the CSS class for selecting background color of card title.
	Footer footer
	// Footer denotes the Election footer

	Voted skipchain.SkipBlockID
	// Voted denotes if a user has already cast a ballot for this election.
}


// footer denotes the fields for the election footer
type footer struct {
	Text         string
	// Text is for storing footer content.
	ContactTitle string
	// ContactTitle stores the title of the Contact person.
	ContactPhone string
	// ContactPhone stores the phone number of the Contact person.
	ContactEmail string
	// ContactEmail stores the email address of the Contact person.
}


// Ballot represents an encrypted vote.
type Ballot struct {
	User uint32
	// User identifier.

	// ElGamal ciphertext pair.
	Alpha kyber.Point
	Beta  kyber.Point
}

// Mix contains the shuffled ballots.
type Mix struct {
	Ballots []*Ballot
	// Ballots are permuted and re-encrypted.
	Proof   []byte
	// Proof of the shuffle.

	NodeID    network.ServerIdentityID
	// Node signifies the creator of the mix.
	Signature []byte
	// Signature of the public key
}

// Partial contains the partially decrypted ballots.
type Partial struct {
	Points []kyber.Point
	// Points are the partially decrypted plaintexts.

	NodeID    network.ServerIdentityID
	// NodeID is the node having signed the partial
	Signature []byte
	// Signature of the public key
}

// Master is the foundation object of the entire service.
// It contains mission critical information that can only be accessed and
// set by an administrators.
type Master struct {
	ID     skipchain.SkipBlockID
	// ID is the hash of the genesis skipblock.
	Roster *onet.Roster
	// Roster is the set of responsible conodes.

	Admins []uint32
	// Admins is the list of administrators.

	Key kyber.Point
	// Key is the front-end public key.
}

// Link is a wrapper around the genesis Skipblock identifier of an
// election. Every newly created election adds a new link to the master Skipchain.
type Link struct {
	ID skipchain.SkipBlockID
}


// Transaction is the sole data structure withing the blocks of an election
// skipchain, it holds all the other containers.
type Transaction struct {
	Master *Master
	Link   *Link

	Election *Election
	Ballot   *Ballot
	Mix      *Mix
	Partial  *Partial

	User      uint32
	Signature []byte
}



