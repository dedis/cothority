package sign

import (
	"fmt"
)

// Round  holds the functions that are used to define the
// behaviour of a Round. All different round-types use the
// cothority-tree, but they have different behaviors.
// This is only the interface, so actual implementation can also start new
// rounds of same type or different at the time it want.
type Round interface {
	// Announcement: root -> nodes
	// This is called from the root-node whenever an
	// announcement is made.
	// TODO: remove Node-argument from function - this should be kept as
	// internal variable in CallbackStamper
	Announcement(int, *SigningMessage, []*SigningMessage) error
	// Commitment: nodes -> root
	// This is called whenever a commitment is ready to
	// be sent. It takes the messages of its children and returns
	// the new message to be sent.
	Commitment([]*SigningMessage, *SigningMessage) error
	// Challenge: root -> nodes
	// This is called with the message to be signed. If necessary,
	// each node can change the message for its children.
	Challenge(*SigningMessage, []*SigningMessage) error
	// Response: nodes -> root
	// This is called with the signature of the challenge-message
	// or with updated ExceptionList* in case of refusal to sign.
	Response([]*SigningMessage, *SigningMessage) error
	// SignatureBroadcast: root -> nodes
	// This is called whenever the turn is completed and
	// the results are propagated through the tree.
	// return error if something is wrong and no need to broadcast down the tree
	// return array of sigbroadcast because if we are root we want to put
	// whatever we need inside. Give fine grained control to user as to what
	// final signature is given to which peer.
	SignatureBroadcast(*SigningMessage, []*SigningMessage) error
	// Statistics: nodes -> root
	// This is called at the end to collect eventual statistics
	// about the round.
}

// RoundFactory is a function that returns a Round given a SigningNode
type RoundFactory func(*Node) Round

// RoundFactories holds the different round factories together. Each round has a
// "type name" that can be associated with its RoundFactory
var RoundFactories map[string]RoundFactory

// Init function init the map
func init() {
	RoundFactories = make(map[string]RoundFactory)
}

// RegisterRoundFactory register a new round factory given its name type.
func RegisterRoundFactory(roundType string, rf RoundFactory) {
	RoundFactories[roundType] = rf
}

// Return the RoundFactory for this round type. Return an error if this round
// has not been registered before.
func NewRoundFromType(rtype string, sn *Node) (Round, error) {
	rf, ok := RoundFactories[rtype]
	if !ok {
		return nil, fmt.Errorf("RoundFactory not registered for the type %s", rtype)
	}
	return rf(sn), nil
}
