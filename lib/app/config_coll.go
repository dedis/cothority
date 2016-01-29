package app

type ConfigColl struct {
	*ConfigConode

	// ppm is the replication factor of hosts per node: how many hosts do we want per node
	Ppm int
	// bf is the branching factor of the tree that we build
	Bf int

	// How many messages to send
	Nmsgs int
	// The speed of request stamping/ms
	Rate int
	// Percentage of stamp server we want to request on (0% = only leader)
	StampRatio float64
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

	// How many stamps per round are we signing limiting rate
	// if StampsPerRound == -1 ==> no limits
	StampsPerRound int

	// RootWait - how long the root timestamper waits for the clients to start up
	RootWait int
	// Just set up the connections and then quit
	TestConnect bool
}
