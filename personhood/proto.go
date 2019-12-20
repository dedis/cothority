package personhood

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/personhood/contracts"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
)

// PROTOSTART
// type :skipchain.SkipBlockID:bytes
// type :byzcoin.InstanceID:bytes
// type :darc.ID:bytes
// type :contracts.RoPaSci:personhood.RoPaSci
// type :contracts.CredentialStruct:personhood.CredentialStruct
// package personhood_service;
//
// import "onet.proto";
// import "darc.proto";
// import "personhood.proto";
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "PersonhoodService";

// PartyList can either store a new party in the list, or just return the list of
// available parties.
type PartyList struct {
	NewParty    *Party
	WipeParties *bool
	PartyDelete *PartyDelete
}

// PartyDelete can be sent from one of the admins to remove a party.
type PartyDelete struct {
	PartyID   byzcoin.InstanceID
	Identity  darc.Identity
	Signature []byte
}

// PartyListResponse holds a list of all known parties so far. Only parties in PreBarrier
// state are listed.
type PartyListResponse struct {
	Parties []Party
}

// Party represents everything necessary to find a party in the ledger.
type Party struct {
	// Roster is the list of nodes responsible for the byzcoin instance
	Roster onet.Roster
	// ByzCoinID represents the ledger where the pop-party is stored.
	ByzCoinID skipchain.SkipBlockID
	// InstanceID is where to find the party in the ledger.
	InstanceID byzcoin.InstanceID
}

// RoPaSciList can either store a new RockPaperScissors in the list, or just
// return the available RoPaScis.
type RoPaSciList struct {
	NewRoPaSci *contracts.RoPaSci
	Wipe       *bool
	// RoPaSciLock allows to ask to lock a ropasci-game and take 1 minute to reply.
	// After 1 minute, the game is again released. If the given game is not available,
	// another one will be presented, when available.
	Lock *contracts.RoPaSci
}

// RoPaSciListResponse returns a list of all known, unfinished RockPaperScissors
// games.
type RoPaSciListResponse struct {
	RoPaScis []contracts.RoPaSci
}

// StringReply can be used by all calls that need a string to be returned
// to the caller.
type StringReply struct {
	Reply string
}

// Poll allows for adding, listing, and answering to storagePolls
type Poll struct {
	ByzCoinID skipchain.SkipBlockID
	NewPoll   *PollStruct
	List      *PollList
	Answer    *PollAnswer
	Delete    *PollDelete
}

// PollDelete has the poll to be deleted, and the signature proving that
// the client has the right to do so.
// The signature is a Schnorr signature on the PollID.
type PollDelete struct {
	Identity  darc.Identity
	PollID    []byte
	Signature []byte
}

// PollList returns all known storagePolls for this byzcoinID
type PollList struct {
	PartyIDs []byzcoin.InstanceID
}

// PollAnswer stores one answer for a poll. It needs to be signed with a Linkable Ring Signature
// to proof that the choice is unique. The context for the LRS must be
//   'Poll' + ByzCoinID + PollID
// And the message must be
//   'Choice' + byte(Choice)
type PollAnswer struct {
	PollID  []byte
	Choice  int
	LRS     []byte
	PartyID byzcoin.InstanceID `protobuf:"opt"`
}

// PollStruct represents one poll with answers.
type PollStruct struct {
	Personhood  byzcoin.InstanceID
	PollID      []byte `protobuf:"opt"`
	Title       string
	Description string
	Choices     []string
	Chosen      []PollChoice `protobuf:"opt"`
}

// PollChoice represents one choice of one participant.
type PollChoice struct {
	Choice int
	LRSTag []byte
}

// PollResponse is sent back to the client and contains all storagePolls known that
// still have a reward left. It also returns the coinIID of the pollservice
// itself.
type PollResponse struct {
	Polls []PollStruct
}

// Capabilities returns what the service is able to do.
type Capabilities struct {
}

// CapabilitiesResponse is the response with the endpoints and the version of each
// endpoint. The versioning is a 24 bit value, that can be interpreted in hexadecimal
// as the following:
//   Version = [3]byte{xx, yy, zz}
//   - xx - major version - incompatible
//   - yy - minor version - downwards compatible. A client with a lower number will be able
//     to interact with this server
//   - zz - patch version - whatever suits you - higher is better, but no incompatibilities
type CapabilitiesResponse struct {
	Capabilities []Capability
}

// Capability is one endpoint / version pair
type Capability struct {
	Endpoint string
	Version  [3]byte
}

// UserLocation is the moment a user has been at a certain location.
type UserLocation struct {
	PublicKey     kyber.Point
	CredentialIID *byzcoin.InstanceID
	Credential    *contracts.CredentialStruct
	Location      *string
	Time          int64
}

// Meetup is sent by a user who wants to discover who else is around.
type Meetup struct {
	UserLocation *UserLocation
	Wipe         *bool
}

// MeetupResponse contains all users from the last x minutes.
type MeetupResponse struct {
	Users []UserLocation
}

// Challenge allows a participant to sign up and to fetch the latest list of scores.
type Challenge struct {
	Update *ChallengeCandidate
}

// ChallengeCandidate is the information the client sends to the server.
// Some of the information is not verifiable for the moment (meetups and references).
type ChallengeCandidate struct {
	Credential byzcoin.InstanceID
	Score      int
	Signup     int64
}

// ChallengeReply is sent back to the client and holds a list of pairs of Credential/Score
// to display on the client's phone.
type ChallengeReply struct {
	List []ChallengeCandidate
}

// GetAdminDarcIDs returns a slice of adminDarcs that are allowed to delete the
// polls and add a party.
type GetAdminDarcIDs struct {
}

// GetAdminDarcIDsReply returns the list of adminDarcs that are allowed to
// delete the polls and add a party.
type GetAdminDarcIDsReply struct {
	AdminDarcIDs []darc.ID
}

// SetAdminDarcIDs sets the list of admin darcs.
// The signature must be on
//   sha256( AdminDarcID[0] | AdminDarcID[1] | ... )
type SetAdminDarcIDs struct {
	NewAdminDarcIDs []darc.ID
	Signature       []byte
}

// SetAdminDarcIDsReply indicates a correct storage of the AdminDarcIDs.
type SetAdminDarcIDsReply struct {
}
