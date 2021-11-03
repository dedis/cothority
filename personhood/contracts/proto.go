package contracts

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
)

// PROTOSTART
// type :skipchain.SkipBlockID:bytes
// type :byzcoin.InstanceID:bytes
// type :darc.ID:bytes
// type :CredentialEntry:string
// type :AttributeString:string
// package personhood;
//
// import "byzcoin.proto";
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "Personhood";

// RoPaSci represents one rock-paper-scissors game.
type RoPaSci struct {
	ByzcoinID skipchain.SkipBlockID
	RoPaSciID byzcoin.InstanceID
	Locked    int64 `protobuf:"opt"`
}

// RoPaSciStruct holds one Rock Paper Scissors event
type RoPaSciStruct struct {
	Description         string
	Stake               byzcoin.Coin
	FirstPlayerHash     []byte
	FirstPlayer         int                 `protobuf:"opt"`
	SecondPlayer        int                 `protobuf:"opt"`
	SecondPlayerAccount byzcoin.InstanceID  `protobuf:"opt"`
	FirstPlayerAccount  *byzcoin.InstanceID `protobuf:"opt"`
	CalypsoWrite        *byzcoin.InstanceID `protobuf:"opt"`
	CalypsoRead         *byzcoin.InstanceID `protobuf:"opt"`
}

// CredentialStruct holds a slice of credentials.
type CredentialStruct struct {
	Credentials []Credential
}

// Credential represents one identity of the user.
type Credential struct {
	Name       CredentialEntry
	Attributes []Attribute
}

// Attribute stores one specific attribute of a credential.
type Attribute struct {
	Name  AttributeString
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
	CostValue      *byzcoin.Coin
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
