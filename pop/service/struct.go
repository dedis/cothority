package service

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	for _, msg := range []interface{}{
		CheckConfig{}, CheckConfigReply{},
	} {
		network.RegisterMessage(msg)
	}
}

// CheckConfig asks whether the pop-config and the attendees are available.
type CheckConfig struct {
	PopHash   []byte
	Attendees []abstract.Point
}

// CheckConfigReply sends back an integer for the Pop:
// - 0 - no popconfig yet
// - 1 - popconfig, but other hash
// - 2 - popconfig with the same hash but no attendees in common
// - 3 - popconfig with same hash and at least one attendee in common
// if PopStatus == 3, then the Attendees will be the common attendees between
// the two nodes.
type CheckConfigReply struct {
	PopStatus int
	PopHash   []byte
	Attendees []abstract.Point
}
