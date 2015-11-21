package conode
import (
	"github.com/dedis/cothority/lib/sign"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
)

/*
Implements a test-round which uses RoundCosi and RoundStamp
 */

const RoundCosiStamperType = "cosistamper"

type RoundCosiStamper struct {
	*RoundStamper
	*RoundCosi
	peer *Peer
}

func RegisterRoundCosiStamper(p *Peer) {
	sign.RegisterRoundFactory(RoundCosiStamperType,
		func(s *sign.Node) sign.Round {
			return NewRoundCosiStamper(p)
		})
}

func NewRoundCosiStamper(peer *Peer) *RoundCosiStamper {
	round := &RoundCosiStamper{}
	round.RoundStamper = NewRoundStamper(peer)
	round.RoundCosi = NewRoundCosi(peer)
	round.peer = peer
	return round
}

func (round *RoundCosiStamper) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage,
out []*sign.SigningMessage) error {
	dbg.Lvl3("Starting new announcement")
	round.RoundStamper.Announcement(viewNbr, roundNbr, in, out)
	round.RoundCosi.Announcement(viewNbr, roundNbr, in, out)
	// TODO: this should go away later
	round.RoundCosi.Timestamp = round.RoundStamper.Timestamp
	for i := range (out) {
		out[i].Am.RoundType = RoundCosiStamperType
	}
	return nil
}

func (round *RoundCosiStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	round.peer.Mux.Lock()
	// get data from s once to avoid refetching from structure
	round.RoundStamper.QueueSet(round.peer.Queue)
	round.peer.Mux.Unlock()

	round.RoundStamper.Commitment(in, out)
	// TODO: shouldn't be needed in the end
	round.RoundCosi.Queue = round.RoundStamper.Queue
	round.RoundCosi.StampLeaves = round.RoundStamper.StampLeaves
	round.RoundCosi.StampProofs = round.RoundStamper.StampProofs
	round.RoundCosi.StampRoot = round.RoundStamper.StampRoot
	round.RoundCosi.Commitment(in, out)
	return nil
}

func (round *RoundCosiStamper) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	round.RoundStamper.Challenge(in, out)
	round.RoundCosi.Challenge(in, out)
	return nil
}

func (round *RoundCosiStamper) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	round.RoundStamper.Response(in, out)
	round.RoundCosi.Response(in, out)
	return nil
}

func (round *RoundCosiStamper) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	round.RoundCosi.SignatureBroadcast(in, out)
	round.RoundStamper.Cosi = round.RoundCosi.Cosi
	round.RoundStamper.SignatureBroadcast(in, out)
	return nil
}
