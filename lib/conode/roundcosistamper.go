package conode
import (
	"github.com/dedis/cothority/lib/sign"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
)

/*
Implements a Stamper and a Cosi-round
 */

const RoundCosiStamperType = "cosistamper"

type RoundCosiStamper struct {
	*sign.RoundCosi
	*sign.RoundStruct
	*RoundStamper
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
	round.RoundCosi = sign.NewRoundCosi(peer.Node)
	round.RoundStruct = sign.NewRoundStruct(peer.Node)
	round.peer = peer
	return round
}

func (round *RoundCosiStamper) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage,
out []*sign.SigningMessage) error {
	dbg.Lvl3("Starting new announcement")
	round.RoundStamper.Announcement(viewNbr, roundNbr, in, out)
	round.RoundCosi.Announcement(viewNbr, roundNbr, in, out)
	round.RoundStruct.SetRoundType(RoundCosiStamperType, out)
	return nil
}

func (round *RoundCosiStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	round.peer.Mux.Lock()
	// get data from s once to avoid refetching from structure
	round.RoundStamper.QueueSet(round.peer.Queue)
	round.peer.Mux.Unlock()

	round.RoundStamper.Commitment(in, out)
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
	round.RoundStamper.Proof = round.RoundCosi.Cosi.Proof
	round.RoundStamper.MTRoot = round.RoundCosi.Cosi.MTRoot
	round.RoundStamper.SignatureBroadcast(in, out)
	return nil
}
