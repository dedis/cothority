package service

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/onet/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	for _, msg := range []interface{}{
		CheckConfig{}, CheckConfigReply{},
		StoreConfig{}, StoreConfigReply{},
		PinRequest{},
		FinalizeRequest{}, FinalizeResponse{},
	} {
		network.RegisterPacketType(msg)
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

// StoreConfig presents a config to store
// TODO: sign this with the private key of the linked app
type StoreConfig struct {
	Desc *PopDesc
}

// TODO: StoreConfigReply will give in a later version a handler that can be used to
// identify that config.
type StoreConfigReply struct {
	ID []byte
}

// PingRequest will print a random pin on stdout if the pin is empty. If
// the pin is given and is equal to the random pin chosen before, the
// public-key is stored as a reference to the allowed client.
type PinRequest struct {
	Pin    string
	Public abstract.Point
}

// FinalizeRequest asks to finalize on the given descid-popconfig.
// TODO: support more than one popconfig
type FinalizeRequest struct {
	DescID    []byte
	Attendees []abstract.Point
}

// FinalizeResponse returns the FinalStatement if all conodes already received
// a PopDesc and signed off. The FinalStatement holds the updated PopDesc, the
// pruned attendees-public-key-list and the collective signature.
type FinalizeResponse struct {
	Final *FinalStatement
}
