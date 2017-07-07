package randhound

import (
	"github.com/dedis/cothority/randhound/protocol"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/network"
)

// Register messages to the network.
func init() {
	network.RegisterMessage(SetupRequest{})
	network.RegisterMessage(SetupReply{})
	network.RegisterMessage(RandRequest{})
	network.RegisterMessage(RandReply{})
}

// SetupRequest is a message sent to a conode asking for the instantiation of a
// Pulsar service with the given parameters.
type SetupRequest struct {
	Roster   *onet.Roster // List of nodes
	Groups   int          // Number of sub-groups
	Purpose  string       // Purpose of the randomness
	Interval int          // Interval time (in millieseconds) between two random values
}

// SetupReply is sent once the Pulsar service was set up successfully.
type SetupReply struct {
}

// RandRequest is a message sent from a client to a randomness service to request collective randomness.
type RandRequest struct {
}

// RandReply is a message sent from a randomness service to a client to return collective randomness.
type RandReply struct {
	R []byte               // Collective randomness
	T *protocol.Transcript // RandHound protocol transcript
}
