package lib

import (
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
)

// PROTOSTART
// package evoting;
// type :skipchain.SkipBlockID:bytes
// type :map\[string\]string:map<string, string>
// type :network.ServerIdentityID:bytes
// type :ElectionState:uint32
// import "onet.proto";
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "Evoting";

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

// Master is the foundation object of the entire service.
// It contains mission critical information that can only be accessed and
// set by an administrators.
type Master struct {
	// ID is the hash of the genesis skipblock.
	ID skipchain.SkipBlockID
	// Roster is the set of responsible conodes.
	Roster *onet.Roster

	// Admins is the list of administrators.
	Admins []uint32

	// Key is the front-end public key.
	Key kyber.Point
}

// Link is a wrapper around the genesis Skipblock identifier of an
// election. Every newly created election adds a new link to the master Skipchain.
type Link struct {
	ID skipchain.SkipBlockID
}

// Election is the base object for a voting procedure. It is stored
// in the second skipblock right after the (empty) genesis block. A reference
// to the election skipchain is appended to the master skipchain upon opening.
type Election struct {
	// Name of the election. lang-code, value pair
	Name map[string]string
	// Creator is the election responsible.
	Creator uint32
	// Users is the list of registered voters.
	Users []uint32

	// ID is the hash of the genesis block.
	ID skipchain.SkipBlockID
	// Master is the hash of the master skipchain.
	Master skipchain.SkipBlockID
	// Roster is the set of responsible nodes.
	Roster *onet.Roster
	// Key is the DKG public key.
	Key kyber.Point
	// MasterKey is the front-end public key.
	MasterKey kyber.Point
	// Stage indicates the phase of election and is used for filtering in frontend
	Stage ElectionState

	// Candidates is the list of candidate scipers.
	Candidates []uint32
	// MaxChoices is the max votes in allowed in a ballot.
	MaxChoices int
	// Description in string format. lang-code, value pair
	Subtitle map[string]string
	// MoreInfo is the url to AE Website for the given election.
	MoreInfo string
	// Start denotes the election start unix timestamp
	Start int64
	// End (termination) datetime as unix timestamp.
	End int64

	// Theme denotes the CSS class for selecting background color of card title.
	Theme string
	// Footer denotes the Election footer
	Footer Footer

	// Voted denotes if a user has already cast a ballot for this election.
	Voted skipchain.SkipBlockID
	// MoreInfoLang, is MoreInfo, but as a lang-code/value map. MoreInfoLang should be used in preference to MoreInfo.
	MoreInfoLang map[string]string
}

// Footer denotes the fields for the election footer
type Footer struct {
	// Text is for storing footer content.
	Text string
	// ContactTitle stores the title of the Contact person.
	ContactTitle string
	// ContactPhone stores the phone number of the Contact person.
	ContactPhone string
	// ContactEmail stores the email address of the Contact person.
	ContactEmail string
}

// Ballot represents an encrypted vote.
type Ballot struct {
	// User identifier.
	User uint32

	// ElGamal ciphertext pair.
	Alpha kyber.Point
	Beta  kyber.Point
}

// Mix contains the shuffled ballots.
type Mix struct {
	// Ballots are permuted and re-encrypted.
	Ballots []*Ballot
	// Proof of the shuffle.
	Proof []byte

	// Node signifies the creator of the mix.
	NodeID network.ServerIdentityID
	// Signature of the public key
	Signature []byte
}

// Partial contains the partially decrypted ballots.
type Partial struct {
	// Points are the partially decrypted plaintexts.
	Points []kyber.Point

	// NodeID is the node having signed the partial
	NodeID network.ServerIdentityID
	// Signature of the public key
	Signature []byte
}
