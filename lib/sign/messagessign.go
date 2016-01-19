package sign

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/suites"
)

/*
All message structures defined in this package are used in the
Collective Signing Protocol
Over the network they are sent as byte slices, so each message
has its own MarshalBinary and UnmarshalBinary method
*/

var Announcement = network.RegisterMessageType(AnnouncementMessage{})
var Commitment = network.RegisterMessageType(CommitmentMessage{})
var Challenge = network.RegisterMessageType(ChallengeMessage{})
var Response = network.RegisterMessageType(ResponseMessage{})
var SignatureBroadcast = network.RegisterMessageType(SignatureBroadcastMessage{})
var StatusReturn = network.RegisterMessageType(StatusReturnMessage{})
var CatchUpReq = network.RegisterMessageType(CatchUpRequest{})
var CatchUpResp = network.RegisterMessageType(CatchUpResponse{})
var GroupChanged = network.RegisterMessageType(GroupChangedMessage{})
var VoteRequest = network.RegisterMessageType(VoteRequestMessage{})
var CloseAll = network.RegisterMessageType(CloseAllMessage{})
var Identity = network.RegisterMessageType(IdentityMessage{})

// IdentityMessage is used when we connect to a node listening, we must give to
// him our identity which is our listeningaddress:port, the same as in the
// tree used.
type IdentityMessage struct {
	PeerName string
}

type SigningMessage struct {
	To           string
	ViewNbr      int
	LastSeenVote int // highest vote ever seen and commited in log, used for catch-up
	RoundNbr     int
	From         string
	Empty        bool // when the application message type  == DefaulType,
}

// Empty struct just to notify to close
type CloseAllMessage SigningMessage

func GetSuite(suite string) abstract.Suite {
	s, ok := suites.All()[suite]
	if !ok {
		dbg.Lvl1("Suites available:", suites.All())
		dbg.Fatal("Didn't find suite", suite)
	}
	return s
}

func NewSigningMessage() interface{} {
	return &SigningMessage{}
}

// Broadcasted message initiated and signed by proposer
type AnnouncementMessage struct {
	*SigningMessage
	Message   []byte
	RoundType string // what kind of round this announcement is made for
	// VoteRequest *VoteRequest
	Vote *Vote // Vote Request (propose)
}

// Commitment of all nodes together with the data they want
// to have signed
type CommitmentMessage struct {
	*SigningMessage
	Message []byte
	V       abstract.Point // commitment Point
	V_hat   abstract.Point // product of subtree participating nodes' commitment points
	X_hat   abstract.Point // product of subtree participating nodes' public keys

	MTRoot hashid.HashId // root of Merkle (sub)Tree

	// public keys of children servers that did not respond to
	// annoucement from root
	RejectionPublicList []abstract.Point

	// CountedVotes *CountedVotes // CountedVotes contains a subtree's votes
	Vote *Vote // Vote Response (promise)

	Messages int // Actual number of messages signed
}

// The challenge calculated by the root-node
type ChallengeMessage struct {
	*SigningMessage
	Message []byte
	C       abstract.Secret // challenge

	// Depth  byte
	MTRoot hashid.HashId // the very root of the big Merkle Tree
	Proof  proof.Proof   // Merkle Path of Proofs from root to us

	// CountedVotes *CountedVotes //  CountedVotes contains the whole tree's votes
	Vote *Vote // Vote Confirmerd/ Rejected (accept)

}

// Every node replies with eventual exceptions if they
// are not OK
type ResponseMessage struct {
	*SigningMessage
	Message []byte
	R_hat   abstract.Secret // response

	// public keys of children servers that did not respond to
	// challenge from root
	RejectionPublicList []abstract.Point
	// nodes that refused to commit:
	RejectionCommitList []abstract.Point

	// cummulative point commits of nodes that failed after commit
	ExceptionV_hat abstract.Point
	// cummulative public keys of nodes that failed after commit
	ExceptionX_hat abstract.Point

	Vote *Vote // Vote Ack/Nack in thr log (ack/nack)

}

// 5th message going from root to leaves to send the
// signature
type SignatureBroadcastMessage struct {
	*SigningMessage
	// Aggregate response of root
	R0_hat abstract.Secret
	// Challenge
	C abstract.Secret
	// Aggregate public key
	X0_hat abstract.Point
	// Aggregate public commitment
	V0_hat abstract.Point
	// challenge from root
	RejectionPublicList []abstract.Point
	RejectionCommitList []abstract.Point
	// Number of messages signed
	Messages int
}

// StatusReturnMessage carries the last status after the
// SignatureBroadcastMessage has been sent to everybody.
// Every node should just add up the stats from its children.
type StatusReturnMessage struct {
	*SigningMessage
	// How many nodes sent a 'respond' message
	Responders int
	// How many peers contacted for a challenge
	Peers int
}

// In case of an error, this message is sent
type ErrorMessage struct {
	*SigningMessage
	Err string
}

// For request of a vote on tree-structure change
type VoteRequestMessage struct {
	*SigningMessage
	Vote *Vote
}

// Whenever the group changed
type GroupChangedMessage struct {
	*SigningMessage
	V *Vote
	// if vote not accepted rest of fields are nil
	HostList []string
}
