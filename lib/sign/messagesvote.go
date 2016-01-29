package sign

import (
	"reflect"
	"sync"
	"time"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/nist"
	"github.com/dedis/protobuf"
)

type VoteType int

const (
	DefaultVT VoteType = iota
	ViewChangeVT
	AddVT
	RemoveVT
	ShutdownVT
	NoOpVT
)

// Multi-Purpose Vote embeds Action to be voted on, aggregated votes, and decison
// when embedded in Announce it equals Vote Request (propose)
// when embedded in Commit it equals Vote Response (promise)
// when embedded in Challenge it equals Vote Confirmed (accept)
// when embedded in Response it equals Vote Ack/ Nack (ack/ nack)
type Vote struct {
	Index int
	View  int
	Round int

	Type VoteType
	Av   *AddVote
	Rv   *RemoveVote
	Vcv  *ViewChangeVote

	Count     *Count
	Confirmed bool
}

type ViewChangeVote struct {
	View   int    // view number we want to switch to
	Parent string // our parent currently
	Root   string // the root for the new view
	// TODO: potentially have signature of new root on proposing this view
}

type AddVote struct {
	View   int    // view number when we want add to take place
	Name   string // who we want to add
	Parent string // our parent currently
}

type RemoveVote struct {
	View   int    // view number when we want add to take place
	Name   string // who we want to remove
	Parent string // our parent currently
}

type VoteResponse struct {
	Name     string // name of the responder
	Accepted bool
	// signature proves ownership of vote and
	// shows that it was emitted during a specifc Round
	Sig BasicSig
}

// A basic, verifiable signature
type BasicSig struct {
	C abstract.Secret // challenge
	R abstract.Secret // response
}

// for sorting arrays of VoteResponse
type ByVoteResponse []*VoteResponse

func (vr ByVoteResponse) Len() int           { return len(vr) }
func (vr ByVoteResponse) Swap(i, j int)      { vr[i], vr[j] = vr[j], vr[i] }
func (vr ByVoteResponse) Less(i, j int) bool { return (vr[i].Name < vr[j].Name) }

// When sent up in a Committment Message CountedVotes contains a subtree's votes
// When sent down in a Challenge Message CountedVotes contains the whole tree's votes
type Count struct {
	Responses []*VoteResponse // vote responses from descendants
	For       int             // number of votes for
	Against   int             // number of votes against
}

type CatchUpRequest struct {
	*SigningMessage
	Index int // index of requested vote
}

type CatchUpResponse struct {
	*SigningMessage
	Vote *Vote
}

func (v *Vote) MarshalBinary() ([]byte, error) {
	return protobuf.Encode(v)
}

func (v *Vote) UnmarshalBinary(data []byte) error {
	var cons = make(protobuf.Constructors)
	var point abstract.Point
	var secret abstract.Secret
	var suite = nist.NewAES128SHA256P256()
	cons[reflect.TypeOf(&point).Elem()] = func() interface{} { return suite.Point() }
	cons[reflect.TypeOf(&secret).Elem()] = func() interface{} { return suite.Secret() }
	return protobuf.DecodeWithConstructors(data, v, cons)
}

type VoteLog struct {
	Entries []*Vote
	Last    int // last set entry

	mu sync.Mutex
}

func (vl *VoteLog) Put(index int, v *Vote) {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	for index >= len(vl.Entries) {
		buf := make([]*Vote, len(vl.Entries)+1)
		vl.Entries = append(vl.Entries, buf...)
	}
	vl.Entries[index] = v

	vl.Last = max(vl.Last, index)
}

func (vl *VoteLog) Get(index int) *Vote {
	if index >= len(vl.Entries) {
		return nil
	}

	return vl.Entries[index]
}

func NewVoteLog() *VoteLog {
	return &VoteLog{Last: -1}
}

func (vl *VoteLog) Stream() chan *Vote {
	ch := make(chan *Vote, 0)
	go func() {

		i := 1
		for {
			vl.mu.Lock()
			v := vl.Get(i)
			vl.mu.Unlock()
			if v != nil {
				ch <- v
				i++
			} else {
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()
	return ch
}
