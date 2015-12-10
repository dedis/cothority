package main

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sign"
)

// The name type of this round implementation
const RoundSkeletonType = "empty"

// RoundSkeleton is the barebone struct that will be used for a round.
// You can inherit of some already implemented rounds such as roundcosi, or
// roundexception etc. You should read and understand the code of the round you are embedding
// in your structs.
type RoundSkeleton struct {
	// RoundCosi is the basis of the Schnorr signature protocol. It will create
	// the commitments, the challenge, the responses, verify all is in order
	// etc. For this version of the API, You have to embed this round and call
	// the appropriate methods in each phase of a round. NOTE that many changes
	// will be done on the API, notably to change to a middleware approach.
	*sign.RoundCosi
	// This measure is used to measure the time of a round. You can have many to
	// measure precise time of a phase of a round or what you want.
	// NOTE: for the moment we need to have a measure, because the way things
	// are done, a simulation is finished and closed when the monitor have
	// received an END connection, when we notify the monitor we have finished
	// our experiment. So we have to notify the monitor process at least for the
	// root that we have finished our experiment at the end
	measure monitor.Measure
}

// Your New Round function
func NewRoundSkeleton(node *sign.Node) *RoundSkeleton {
	dbg.Lvl3("Making new RoundSkeleton", node.Name())
	round := &RoundSkeleton{}
	// You've got to initialize the roundcosi with the node
	round.RoundCosi = sign.NewRoundCosi(node)
	round.Type = RoundSkeletonType
	return round
}

// The first phase is the announcement phase.
// For all phases, the signature is the same, it takes sone Input message and
// Output messages and returns an error if something went wrong.
// For announcement we just give for now the viewNbr (view = what is in the tree
// at the instant) and the round number so we know where/when are we in the run.
func (round *RoundSkeleton) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return round.RoundCosi.Announcement(viewNbr, roundNbr, in, out)
}

// Commitment phase
func (round *RoundSkeleton) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	return round.RoundCosi.Commitment(in, out)
}

// Challenge phase
func (round *RoundSkeleton) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return round.RoundCosi.Challenge(in, out)
}

// Challenge phase
func (round *RoundSkeleton) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	return round.RoundCosi.Response(in, out)
}

// SignatureBroadcast phase
// Here you get your final signature !
func (round *RoundSkeleton) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return round.RoundCosi.SignatureBroadcast(in, out)
}
