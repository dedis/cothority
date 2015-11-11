package app

import (
	"github.com/dedis/cothority/lib/graphs"
)

type ConfigColl struct {
	// ppm is the replication factor of hosts per node: how many hosts do we want per node
	Ppm int
	// bf is the branching factor of the tree that we build
	Bf int
	// Coding-suite to run 	[nist256, nist512, ed25519]
	Suite string

	// How many messages to send
	Nmsgs int
	// The speed of request stamping/ms
	Rate int
	// Percentage of stamp server we want to request on (0% = only leader)
	StampPerc int
	// How many rounds
	Rounds int
	// Pre-defined failure rate
	Failures int
	// Rounds for root to wait before failing
	RFail int
	// Rounds for follower to wait before failing
	FFail int
	// Debug-level
	Debug int

	// RootWait - how long the root timestamper waits for the clients to start up
	RootWait int
	// Just set up the connections and then quit
	TestConnect bool

	// Tree for knowing whom to connect
	Tree *graphs.Tree
	// All hostnames concatenated with the port-number to use
	Hosts []string
}
