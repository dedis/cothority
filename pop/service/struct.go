package service

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"sort"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/onet/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	network.RegisterMessages(CheckConfig{}, CheckConfigReply{},
		PinRequest{}, FetchRequest{}, MergeRequest{},
		StoreConfig{}, StoreConfigReply{},
		GetProposals{}, GetProposalsReply{})
}

func newMerge() *merge {
	mm := &merge{}
	mm.statementsMap = make(map[string]*FinalStatement)
	mm.distrib = false
	return mm
}

type byIdentity []*network.ServerIdentity

func (p byIdentity) Len() int      { return len(p) }
func (p byIdentity) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p byIdentity) Less(i, j int) bool {
	return p[i].String() < p[j].String()
}

type byPoint []kyber.Point

func (p byPoint) Len() int      { return len(p) }
func (p byPoint) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p byPoint) Less(i, j int) bool {
	return p[i].String() < p[j].String()
}

func sortAll(locs []string, roster []*network.ServerIdentity, atts []kyber.Point) {
	sort.Strings(locs)
	sort.Sort(byIdentity(roster))
	sort.Sort(byPoint(atts))
}

// sliceToArr does what the name suggests, we need it to turn a slice into
// something hashable.
func sliceToArr(msg []byte) [32]byte {
	var arr [32]byte
	copy(arr[:], msg)
	return arr
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

// CheckConfig asks whether the pop-config and the attendees are available.
type CheckConfig struct {
	PopHash   []byte
	Attendees []kyber.Point
}

// CheckConfigReply sends back an integer for the Pop. 0 means no config yet,
// other values are defined as constants.
// If PopStatus == PopStatusOK, then the Attendees will be the common attendees between
// the two nodes.
type CheckConfigReply struct {
	PopStatus int
	PopHash   []byte
	Attendees []kyber.Point
}

// MergeConfig asks if party is ready to merge
type MergeConfig struct {
	// FinalStatement of current party
	Final *FinalStatement
	// Hash of PopDesc party to merge with
	ID []byte
}

// MergeConfigReply responds with info of asked party
type MergeConfigReply struct {
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
	Public kyber.Point
}

// StoreConfig presents a config to store
type StoreConfig struct {
	Desc      *PopDesc
	Signature []byte
}

// StoreConfigReply gives back the hash.
// TODO: StoreConfigReply will give in a later version a handler that can be used to
// identify that config.
type StoreConfigReply struct {
	ID []byte
}

// FinalizeRequest asks to finalize on the given descid-popconfig.
type FinalizeRequest struct {
	DescID    []byte
	Attendees []kyber.Point
	Signature []byte
}

func (fr *FinalizeRequest) hash() ([]byte, error) {
	h := cothority.Suite.Hash()
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

// FinalizeResponse returns the FinalStatement if all conodes already received
// a PopDesc and signed off. The FinalStatement holds the updated PopDesc, the
// pruned attendees-public-key-list and the collective signature.
type FinalizeResponse struct {
	Final *FinalStatement
}

// FetchRequest asks to get FinalStatement
type FetchRequest struct {
	ID []byte
}

// MergeRequest asks to start merging process for given Party
type MergeRequest struct {
	ID        []byte
	Signature []byte
}

// GetProposals asks the conode to return a list of all waiting proposals. A waiting
// proposal is either deleted after 1h or if it has been confirmed using
// StoreConfig.
type GetProposals struct {
}

// GetProposalsReply returns the list of all waiting proposals on that node.
type GetProposalsReply struct {
	Proposals []PopDesc
}
