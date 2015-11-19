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
	server := conode.NewPeer()
	// Listens on port 2001 for eventual RPC-calls
	rpc = conode.NewRPC()

	for i := 1; i < 10; i++ {
		dbg.Lvl1("Starting round", i)
		server.startAnnouncement(NewRoundPony(server))
	}
}

// roundPony is our round which we initialise with the conode.RoundMerkle
// structure to have automatic setup of a merkle-tree
type roundPony struct {
	conode.RoundMerkle
	time monitor.Measure
}

// NewRoundPony creates a new round and calls the setup
func NewRoundPony(server conode.Server) *conode.Round {
	round := roundPony{}
	round.Setup(server)
	return round
}

// Setup has to call the included round-structure setup.
func (r *roundPony)Setup(server conode.Server) {
	r.RoundMerkle.Setup(server)
}

// These three structures are optional - they are not needed for a very
// simple merkle-tree. But this very simple Merkle-tree will not do very much
// than send around empty messages

// Announcement prepares the collection of the round-time
func (r *roundPony)Announcement(in *conode.AnnouncementMessage,
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
func (r *roundPony)Commit(in []*conode.ChallengeMessage,
out *conode.ChallengeMessage) error{
	err := r.RoundMerkle.Commit(in, out)
	if err != nil{
		return err
	}

	round.Message, err = rpc.CollectMessages()
	return err
}

// Response measures the round-time
func (r *roundPony)Response(in []*conode.ResponseMessage,
out *conode.ResponseMessage) error{
	err := r.RoundMerkle.Response(in, out)
	if err != nil{
		return err
	}

	if app.RunFlags.AmRoot{
		r.time.Measure()
	}
	return nil
}