package service

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	network.RegisterMessages(CheckConfig{}, CheckConfigReply{},
		PinRequest{}, FetchRequest{}, MergeRequest{},
		StoreConfig{}, StoreConfigReply{},
		GetProposals{}, GetProposalsReply{},
		VerifyLink{}, VerifyLinkReply{},
		PopPartyInstance{}, StoreInstanceID{},
		StoreInstanceIDReply{},
		GetInstanceID{}, GetInstanceIDReply{})
}

// PROTOSTART
// package pop;
// type :map\[string\]FinalStatement:map<string, FinalStatement>
// type :byzcoin.InstanceID:bytes
// import "onet.proto";
// import "darc.proto";
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "PoPProto";

// ShortDesc represents Short Description of Pop party
// Used in merge configuration
type ShortDesc struct {
	Location string
	Roster   *onet.Roster
}

// PopDesc holds the name, date and a roster of all involved conodes.
type PopDesc struct {
	// Name and purpose of the party.
	Name string
	// DateTime of the party. It is in the following format, following UTC:
	//   YYYY-MM-DD HH:mm
	DateTime string
	// Location of the party
	Location string
	// Roster of all responsible conodes for that party.
	Roster *onet.Roster
	// List of parties to be merged
	Parties []*ShortDesc
}

// FinalStatement is the final configuration holding all data necessary
// for a verifier.
type FinalStatement struct {
	// Desc is the description of the pop-party.
	Desc *PopDesc
	// Attendees holds a slice of all public keys of the attendees.
	Attendees []kyber.Point
	// Signature is created by all conodes responsible for that pop-party
	Signature []byte
	// Flag indicates that party was merged
	Merged bool
}

// CheckConfig asks whether the pop-config and the attendees are available.
type CheckConfig struct {
	PopHash   []byte
	Attendees []kyber.Point
}

// CheckConfigReply sends back an integer for the Pop. 0 means no config yet,
// other values are defined as constants.
// If PopStatus == PopStatusOK, then the Attendees will be the common attendees between
// the two nodes.
type CheckConfigReply struct {
	PopStatus int
	PopHash   []byte
	Attendees []kyber.Point
}

// MergeConfig asks if party is ready to merge
type MergeConfig struct {
	// FinalStatement of current party
	Final *FinalStatement
	// Hash of PopDesc party to merge with
	ID []byte
}

// MergeConfigReply responds with info of asked party
type MergeConfigReply struct {
	// status of merging process
	PopStatus int
	// hash of party was asking to merge
	PopHash []byte
	// FinalStatement of party was asked to merge
	Final *FinalStatement
}

// PinRequest will print a random pin on stdout if the pin is empty. If
// the pin is given and is equal to the random pin chosen before, the
// public-key is stored as a reference to the allowed client.
type PinRequest struct {
	Pin    string
	Public kyber.Point
}

// StoreConfig presents a config to store
type StoreConfig struct {
	Desc      *PopDesc
	Signature []byte
}

// StoreConfigReply gives back the hash.
// TODO: StoreConfigReply will give in a later version a handler that can be used to
// identify that config.
type StoreConfigReply struct {
	ID []byte
}

// FinalizeRequest asks to finalize on the given descid-popconfig.
type FinalizeRequest struct {
	DescID    []byte
	Attendees []kyber.Point
	Signature []byte
}

// FinalizeResponse returns the FinalStatement if all conodes already received
// a PopDesc and signed off. The FinalStatement holds the updated PopDesc, the
// pruned attendees-public-key-list and the collective signature.
type FinalizeResponse struct {
	Final *FinalStatement
}

// FetchRequest asks to get FinalStatement
type FetchRequest struct {
	ID               []byte
	ReturnUncomplete *bool
}

// MergeRequest asks to start merging process for given Party
type MergeRequest struct {
	ID        []byte
	Signature []byte
}

// GetProposals asks the conode to return a list of all waiting proposals. A waiting
// proposal is either deleted after 1h or if it has been confirmed using
// StoreConfig.
type GetProposals struct {
}

// GetProposalsReply returns the list of all waiting proposals on that node.
type GetProposalsReply struct {
	Proposals []PopDesc
}

// VerifyLink returns if a given public key is linked.
type VerifyLink struct {
	Public kyber.Point
}

// VerifyLinkReply returns true if the public key is in the admin-list.
type VerifyLinkReply struct {
	Exists bool
}

// GetLink returns the public key of the linked organizer.
type GetLink struct {
}

// GetLinkReply holds the public key of the linked organizer.
type GetLinkReply struct {
	Public kyber.Point
}

// GetFinalStatements returns all stored final statements.
type GetFinalStatements struct {
}

// GetFinalStatementsReply returns all stored final statements.
type GetFinalStatementsReply struct {
	FinalStatements map[string]*FinalStatement
}

// StoreInstanceID writes an InstanceID from ByzCoin to a FinalStatement.
type StoreInstanceID struct {
	PartyID    []byte
	InstanceID byzcoin.InstanceID
}

// StoreInstanceIDReply is an empty reply
type StoreInstanceIDReply struct {
}

// GetInstanceID requests an InstanceID from ByzCoin to a FinalStatement.
type GetInstanceID struct {
	PartyID []byte
}

// GetInstanceIDReply is the InstanceID for the party
type GetInstanceIDReply struct {
	InstanceID byzcoin.InstanceID
}

// StoreSigner writes an Signer from ByzCoin to a FinalStatement.
type StoreSigner struct {
	PartyID []byte
	Signer  darc.Signer
}

// StoreSignerReply is an empty reply
type StoreSignerReply struct {
}

// GetSigner requests an Signer from ByzCoin to a FinalStatement.
type GetSigner struct {
	PartyID []byte
}

// GetSignerReply is the Signer for the party
type GetSignerReply struct {
	Signer darc.Signer
}

// StoreKeys stores a list of keys for attendees to retrieve
// later.
type StoreKeys struct {
	// ID is the ID of the party where we want to store intermediate keys
	ID []byte
	// Keys is a list of public keys to store
	Keys []kyber.Point
	// Signature proves that the organizer updated the keys
	Signature []byte
}

// StoreKeysReply is an empty message.
type StoreKeysReply struct {
}

// GetKeys can be used to retrieve the keyset for a given party - useful
// for an attendee to know if his key has been scanned.
type GetKeys struct {
	ID []byte
}

// GetKeysReply returns the keys stored for a given Party-ID.
type GetKeysReply struct {
	ID   []byte
	Keys []kyber.Point
}

// PopPartyInstance is the data that is stored in a pop-party instance.
type PopPartyInstance struct {
	// State has one of the following values:
	// 1: it is a configuration only
	// 2: it is a finalized pop-party
	State int
	// FinalStatement has either only the Desc inside if State == 1, or all fields
	// set if State == 2.
	FinalStatement *FinalStatement
	// Previous is the link to the instanceID of the previous party, it can be
	// nil for the first party.
	Previous byzcoin.InstanceID
	// Next is a link to the instanceID of the next party. It can be
	// nil if there is no next party.
	Next byzcoin.InstanceID
	// Public key of service - can be nil.
	Service kyber.Point `protobuf:"opt"`
}
