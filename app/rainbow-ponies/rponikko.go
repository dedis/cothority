/*
This package is the 'ultimate' implementation on how to use the
conode-library. What I think would be the most simplest use.
*/
package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/monitor"
)

var rpc conode.RPC

// main initialises a new server/peer/whatever that listens on
// port 2000 (or whatever the app-flags tell it to listen to)
// and creates the connections.
func main() {
	app.FlagInit()

	// Creates a new server on port 2000 and starts the connections
	node := conode.NewNode()
	// Listens on port 2001 for eventual RPC-calls
	rpc = NewRPCClient()

	// Register the type of rounds
	sign.RegisterRoundFactory("roundpony", func(*sign.Node) Round {
		return Stack(LoggingMiddleware(node), NewWaitAllMiddleware(node), NewMerkleMiddleware(node)).SetInnerRound(NewPonyRound(server))
	})

	stackedRounds := Stack(LoggingMiddleware(node), NewWaitAllMiddleware(node), NewMerkleMiddleware(node)).SetInnerRound(NewPonyRound(server))
	for i := 1; i < 10; i++ {
		dbg.Lvl1("Starting round", i)
		server.startAnnouncement(stackedRounds)
	}
}

// THe basic interface that a middleware round must implement
// It can still be used as a simple round !!
type RoundMiddleware interface {
	sign.Round
	SetInnerRound(r Round) Round
}

// Will stack middleware one on top of the others
func Stack(roundsmid ...RoundMiddleware) RoundMiddleware {
	r = roundsmid[0]
	for i := range roundsmid[1:] {
		r.SetInnerRound(roundsmid[i])
		r = roundsmid[i]
	}
	return r
}

// Implementations that will wait all messages for commits and resposnes
type WaitAllMiddleware struct {
	Commits []*sign.CommitmentMessage
	node    *sign.Node
	inner   Round
}

// SetInnerRound will set the round to call once all messages are here.
func (wa *WaitAllMiddleware) SetInnerRound(r Round) {
	wa.inner = r
	return wa
}

func (wa *WaitAllMiddleware) Announcement(in *conode.AnnouncementMessage) ([]*conode.AnnouncementMessage, error) {
	return []*conode.AnnouncementMessage{in}
}

func (wa *WaitAllMiddleware) Commitment(in []*conode.CommitmentMessage) (*conode.CommitmentMessage, error) {
	wa.Commits = append(wa.Commits, in...)
	if len(wa.Commits) == wa.Node.Children() {
		return inner(wa.Commits)
	}
	return nil, nil
}
func (wa *WaitAllMiddleware) Challenge() {
	// todo
}
func (wa *WaitAllMiddleware) Responses(in []*conode.ResponseMessage) (*conode.ResponseMessage, error) {
	wa.Responses = append(wa.Responses, in...)
	if len(wa.Responses) == len(wa.Node.Children()) {
		return inner.Response(wa.Responses)
	}
	return nil, nil
}

// roundPony is our round which we initialise with the conode.RoundMerkle
// structure to have automatic setup of a merkle-tree
type roundPony struct {
	conode.RoundMerkle
}

// NewRoundPony creates a new round and calls the setup
func NewRoundPony(server conode.Server) *conode.Round {
	round := roundPony{}
	round.Setup(server)
	return round
}

// Setup has to call the included round-structure setup.
func (r *roundPony) Setup(server conode.Server) {
	r.RoundMerkle.Setup(server)
}

// These three structures are optional - they are not needed for a very
// simple merkle-tree. But this very simple Merkle-tree will not do very much
// than send around empty messages

// Announcement prepares the collection of the round-time
func (r *roundPony) Announcement(in *conode.AnnouncementMessage,
	out []*conode.AnnouncementMessage) error {
	err := r.RoundMerkle.Announcement(in, out)
	if err != nil {
		return err
	}

	if app.RunFlags.AmRoot {
		r.time = monitor.NewMeasure("roundtime")
	}
	return nil
}

// Commit collects the messages received through the rpc and sends them to
// the root-node
func (r *roundPony) Commit(in []*conode.ChallengeMessage,
	out *conode.ChallengeMessage) error {
	err := r.RoundMerkle.Commit(in, out)
	if err != nil {
		return err
	}

	round.Message, err = rpc.CollectMessages()
	return err
}

// Response measures the round-time
func (r *roundPony) Response(in []*conode.ResponseMessage,
	out *conode.ResponseMessage) error {
	err := r.RoundMerkle.Response(in, out)
	if err != nil {
		return err
	}

	if app.RunFlags.AmRoot {
		r.time.Measure()
	}
	return nil
}
