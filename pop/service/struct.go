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
		PinRequest{},
	} {
		network.RegisterMessage(msg)
	}
}

const (
	// PopStatusWrongHash - The different configs in the roster don't have the same hash
	PopStatusWrongHash = iota
	// PopStatusNoAttendees - No common attendees found
	PopStatusNoAttendees
	// PopStatusOK - Everything is OK
	PopStatusOK
)

// CheckConfig asks whether the pop-config and the attendees are available.
type CheckConfig struct {
	PopHash   []byte
	Attendees []abstract.Point
}

// CheckConfigReply sends back an integer for the Pop. 0 means no config yet,
// other values are defined as constants.
// If PopStatus == PopStatusOK, then the Attendees will be the common attendees between
// the two nodes.
type CheckConfigReply struct {
	PopStatus int
	PopHash   []byte
	Attendees []abstract.Point
}

// PinRequest will print a random pin on stdout if the pin is empty. If
// the pin is given and is equal to the random pin chosen before, the
// public-key is stored as a reference to the allowed client.
type PinRequest struct {
	Pin    string
	Public abstract.Point
}

// StoreConfig presents a config to store
// TODO: sign this with the private key of the linked app
type StoreConfig struct {
	Desc *PopDesc
}

// StoreConfigReply gives back the hash.
// TODO: StoreConfigReply will give in a later version a handler that can be used to
// identify that config.
type StoreConfigReply struct {
	ID []byte
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
