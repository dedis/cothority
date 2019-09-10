package personhood

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
)

// PROTOSTART
// type :skipchain.SkipBlockID:bytes
// type :byzcoin.InstanceID:bytes
// type :darc.ID:bytes
// package personhood;
//
// import "byzcoin.proto";
// import "onet.proto";
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "Personhood";

// PartyList can either store a new party in the list, or just return the list of
// available parties.
type PartyList struct {
	NewParty    *Party
	WipeParties *bool
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
	NewRoPaSci *RoPaSci
	Wipe       *bool
}

// RoPaSciListResponse returns a list of all known, unfinished RockPaperScissors
// games.
type RoPaSciListResponse struct {
	RoPaScis []RoPaSci
}

// RoPaSci represents one rock-paper-scissors game.
type RoPaSci struct {
	ByzcoinID skipchain.SkipBlockID
	RoPaSciID byzcoin.InstanceID
}

// StringReply can be used by all calls that need a string to be returned
// to the caller.
type StringReply struct {
	Reply string
}

// RoPaSciStruct holds one Rock Paper Scissors event
type RoPaSciStruct struct {
	Description         string
	Stake               byzcoin.Coin
	FirstPlayerHash     []byte
	FirstPlayer         int                `protobuf:"opt"`
	SecondPlayer        int                `protobuf:"opt"`
	SecondPlayerAccount byzcoin.InstanceID `protobuf:"opt"`
	FirstPlayerAccount  byzcoin.InstanceID `protobuf:"opt"`
	CalypsoWrite        byzcoin.InstanceID `protobuf:"opt"`
	CalypsoRead         byzcoin.InstanceID `protobuf:"opt"`
}

// CredentialStruct holds a slice of credentials.
type CredentialStruct struct {
	Credentials []Credential
}

// Credential represents one identity of the user.
type Credential struct {
	Name       string
	Attributes []Attribute
}

// Attribute stores one specific attribute of a credential.
type Attribute struct {
	Name  string
	Value []byte
}

// SpawnerStruct holds the data necessary for knowing how much spawning
// of a certain contract costs.
type SpawnerStruct struct {
	CostDarc       byzcoin.Coin
	CostCoin       byzcoin.Coin
	CostCredential byzcoin.Coin
	CostParty      byzcoin.Coin
	Beneficiary    byzcoin.InstanceID
	CostRoPaSci    byzcoin.Coin `protobuf:"opt"`
	CostCWrite     *byzcoin.Coin
	CostCRead      *byzcoin.Coin
}

// PopPartyStruct is the data that is stored in a pop-party instance.
type PopPartyStruct struct {
	// State has one of the following values:
	//  1: it is a configuration only
	//  2: scanning in progress
	//  3: it is a finalized pop-party
	State int
	// Organizers is the number of organizers responsible for this party
	Organizers int
	// Finalizations is a slice of darc-identities who agree on the list of
	// public keys in the FinalStatement.
	Finalizations []string
	// Description holds the name, date and location of the party and is available
	// before the barrier point.
	Description PopDesc
	// Attendees is the slice of public keys of all confirmed attendees
	Attendees Attendees
	// Miners holds all tags of the linkable ring signatures that already
	// mined this party.
	Miners []LRSTag
	// How much money to mine
	MiningReward uint64
	// Previous is the link to the instanceID of the previous party, it can be
	// nil for the first party.
	Previous byzcoin.InstanceID `protobuf:"opt"`
	// Next is a link to the instanceID of the next party. It can be
	// nil if there is no next party.
	Next byzcoin.InstanceID `protobuf:"opt"`
}

// PopDesc holds the name, date and a roster of all involved conodes.
type PopDesc struct {
	// Name of the party.
	Name string
	// Purpose of the party
	Purpose string
	// DateTime of the party. It is stored as seconds since the Unix-epoch, 1/1/1970
	DateTime uint64
	// Location of the party
	Location string
}

// FinalStatement is the final configuration holding all data necessary
// for a verifier.
type FinalStatement struct {
	// Desc is the description of the pop-party.
	Desc *PopDesc
	// Attendees holds a slice of all public keys of the attendees.
	Attendees Attendees
}

// Attendees is a slice of points of attendees' public keys.
type Attendees struct {
	Keys []kyber.Point
}

// LRSTag is the tag of the linkable ring signature sent in by a user.
type LRSTag struct {
	Tag []byte
}

// Poll allows for adding, listing, and answering to storagePolls
type Poll struct {
	ByzCoinID skipchain.SkipBlockID
	NewPoll   *PollStruct
	List      *PollList
	Answer    *PollAnswer
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
	PollID []byte
	Choice int
	LRS    []byte
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
	Credential    *CredentialStruct
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
