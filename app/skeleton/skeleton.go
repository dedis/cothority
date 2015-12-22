package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sign"
)

// This file is the first draft to a skeleton app where you have all the
// basics to run your own cothority tree.This includes an exemplary main()
// function which shows how to configure and run a cothority based application.
// It also include a basic Round structure that does nothing yet (up to you).
// This round will be executed for each round of the cothority tree.
// This skeleton is for use with the deploy/ lib, that can deploy on localhost
// or on deterlab. This is not intented to be used as a standalone app. For this
// check the app/conode folder which contains everything to run a standalone
// app. Here all the configuration of the tree, public keys, deployement, etc is
// automatically done. You can make some measurements with the monitor/ library.
// It will create a .csv file in deploy/test_data with the same name of the
// simulation file you wrote. Take a look at some simulation files to get an
// idea on how it is working. Please note that this a first draft for this
// current version of the API and a lot of changes will be brought along the
// next months, so of course there's a lot of things that are not ideal, we know
// that ;).

// To run this skeleton app, go to deploy:
// go build && ./deploy -debug 2 simulations/skeleton.toml

func main() {
	// First, let's read our config
	// You should create your own config in lib/app.
	// TOML is a pretty simple and readable format
	// Whatever information needed, supply it in the simulation/.toml file that
	// will be parsed into your ConfigSkeleton struct.
	conf := &app.ConfigSkeleton{}
	app.ReadConfig(conf)

	// we must know who we are
	if app.RunFlags.Hostname == "" {
		dbg.Fatal("Hostname empty: Abort")
	}

	// Do some common setup
	if app.RunFlags.Mode == "client" {
		app.RunFlags.Hostname = app.RunFlags.Name
	}
	hostname := app.RunFlags.Hostname
	// i.e. we are root
	if hostname == conf.Hosts[0] {
		dbg.Lvlf3("Tree is %+v", conf.Tree)
	}
	dbg.Lvl3(hostname, "Starting to run")

	// Connect to the monitor process. This monitor process is run on your
	// machine and accepts connections from any node, usually you only connect
	// with the root for readability and performance reasons (don't connect to
	// your machine from 8000 nodes .. !)
	if app.RunFlags.Monitor != "" {
		monitor.ConnectSink(app.RunFlags.Monitor)
	} else {
		dbg.Fatal("No logger specified")
	}

	// Here you create a "Peer",that's the struct that will create a new round
	// each seconds and handle other subtleties for you
	peer := conode.NewPeer(hostname, conf.ConfigConode)

	// The root waits everyones to be up
	if app.RunFlags.AmRoot {
		err := peer.WaitRoundSetup(len(conf.Hosts), 5, 2)
		if err != nil {
			dbg.Fatal(err)
		}
		dbg.Lvl1("Starting the rounds")
	}

	// You register by giving the type, and a function that takes a sign.Node in
	// input (basically the underlying protocol) and returns a Round.
	sign.RegisterRoundFactory(RoundSkeletonType,
		func(node *sign.Node) sign.Round {
			return NewRoundSkeleton(node)
		})
	// Here it will create a new round each seconds automatically.
	// If you need more fined grained control, you must implement yourself the
	// conode.Peer struct (it's quite easy).
	peer.LoopRounds(RoundSkeletonType, conf.Rounds)
	// Notify the monitor we finished so that the simulation can be stopped
	monitor.End()
}

// The name type of this round implementation
const RoundSkeletonType = "skeleton"

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
	// receveid an END connection, when we notify the monitor we have finished
	// our experiement. So we have to notifiy the monitor process at least for the
	// root that we have finished our experiment at the end
	measure monitor.Measure
}

// Your New Round function
func NewRoundSkeleton(node *sign.Node) sign.Round {
	dbg.Lvl3("Making new RoundSkeleton", node.Name())
	round := &RoundSkeleton{}
	// You've got to initialize the roundcosi with the node
	round.RoundCosi = sign.NewRoundCosi(node)
	round.Type = RoundSkeletonType
	return round
}

// The first phase is the announcement phase.
// For all phases, the signature is the same, it takes some Input message and
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
