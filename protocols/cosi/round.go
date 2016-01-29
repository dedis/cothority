package cosi

import (
	"fmt"
	"github.com/dedis/cothority/lib/sda"
)

// Round is an interface that represent the four steps in the vanilla CoSi
// protocol.
type Round interface {
	// Announcement: root -> nodes
	// This is called from the root-node whenever an
	// announcement is made.
	Announcement(*AnnouncementMessage, []*AnnouncementMessage) error
	// Commitment: nodes -> root
	// This is called whenever a commitment is ready to
	// be sent. It takes the messages of its children and returns
	// the new message to be sent.
	Commitment([]*CommitmentMessage, *CommitmentMessage) error
	// Challenge: root -> nodes
	// This is called with the message to be signed. If necessary,
	// each node can change the message for its children.
	Challenge(*ChallengeMessage, []*ChallengeMessage) error
	// Response: nodes -> root
	// This is called with the signature of the challenge-message
	// or with updated RejectionPublicList* in case of refusal to sign.
	Response([]*ResponseMessage, *ResponseMessage) error
}

// RoundFactory is a function that returns a Round given a SigningNode
type RoundFactory func(*sda.Node) Round

// RoundFactories holds the different round factories together. Each round has a
// "type name" that can be associated with its RoundFactory
var RoundFactories map[string]RoundFactory

// Init function init the map
func init() {
}

// RegisterRoundFactory register a new round factory given its name type.
func RegisterRoundFactory(roundType string, rf RoundFactory) {
	if RoundFactories == nil {
		RoundFactories = make(map[string]RoundFactory)
	}
	RoundFactories[roundType] = rf
}

// Return the RoundFactory for this round type. Return an error if this round
// has not been registered before.
func NewRoundFromType(rtype string, node *sda.Node) (Round, error) {
	rf, ok := RoundFactories[rtype]
	if !ok {
		return nil, fmt.Errorf("RoundFactory not registered for the type %s", rtype)
	}
	return rf(node), nil
}
