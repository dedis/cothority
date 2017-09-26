package service

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	for _, msg := range []interface{}{
		checkConfig{}, checkConfigReply{},
		PinRequest{}, fetchRequest{}, mergeRequest{},
	} {
		network.RegisterMessage(msg)
	}
}

const (
	// PopStatusWrongHash - The different configs in the roster don't have the same hash
	PopStatusWrongHash = iota
	// PopStatusNoAttendees - No common attendees found
	PopStatusNoAttendees
	// PopStatusMergeError - Error in merge config
	PopStatusMergeError
	// PopStatusMergeNonFinalized - Attempt to merge not finalized party
	PopStatusMergeNonFinalized
	// PopStatusOK - Everything is OK
	PopStatusOK
)

// checkConfig asks whether the pop-config and the attendees are available.
type checkConfig struct {
	PopHash   []byte
	Attendees []abstract.Point
}

// checkConfigReply sends back an integer for the Pop. 0 means no config yet,
// other values are defined as constants.
// If PopStatus == PopStatusOK, then the Attendees will be the common attendees between
// the two nodes.
type checkConfigReply struct {
	PopStatus int
	PopHash   []byte
	Attendees []abstract.Point
}

// mergeConfig asks if party is ready to merge
type mergeConfig struct {
	// FinalStatement of current party
	Final *FinalStatement
	// Hash of PopDesc party to merge with
	ID []byte
}

// mergeConfigReply responds with info of asked party
type mergeConfigReply struct {
	// status of merging process
	PopStatus int
	// hash of party was asking to merge
	PopHash []byte
	// FinalStatement of party was asked to merge
	Final *FinalStatement
}

// PinRequest will print a random pin on stdout if the pin is empty. If
// the pin is given and is equal to the random pin chosen before, the
// public-key is stored as a reference to the allowed client.
type PinRequest struct {
	Pin    string
	Public abstract.Point
}

// storeConfig presents a config to store
type storeConfig struct {
	Desc      *PopDesc
	Signature crypto.SchnorrSig
}

// storeConfigReply gives back the hash.
// TODO: storeConfigReply will give in a later version a handler that can be used to
// identify that config.
type storeConfigReply struct {
	ID []byte
}

// finalizeRequest asks to finalize on the given descid-popconfig.
type finalizeRequest struct {
	DescID    []byte
	Attendees []abstract.Point
	Signature crypto.SchnorrSig
}

func (fr *finalizeRequest) hash() ([]byte, error) {
	h := network.Suite.Hash()
	_, err := h.Write(fr.DescID)
	if err != nil {
		return nil, err
	}
	for _, a := range fr.Attendees {
		b, err := a.MarshalBinary()
		if err != nil {
			return nil, err
		}
		_, err = h.Write(b)
		if err != nil {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}

// finalizeResponse returns the FinalStatement if all conodes already received
// a PopDesc and signed off. The FinalStatement holds the updated PopDesc, the
// pruned attendees-public-key-list and the collective signature.
type finalizeResponse struct {
	Final *FinalStatement
}

// fetchRequest asks to get FinalStatement
type fetchRequest struct {
	ID []byte
}

// mergeRequest asks to start merging process for given Party
type mergeRequest struct {
	ID        []byte
	Signature crypto.SchnorrSig
}
