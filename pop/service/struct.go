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
		GetProposals{}, GetProposalsReply{},
		VerifyLink{}, VerifyLinkReply{})
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

