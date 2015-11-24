package conode
import (
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/dbg"
)

/*
Implements a Stamper and a Cosi-round
 */

const RoundCosiStamperType = "cosistamper"

type RoundCosiStamper struct {
	*sign.RoundCosi
	*sign.RoundStruct
	*RoundStamper
}

func init() {
	sign.RegisterRoundFactory(RoundCosiStamperType,
		func(node *sign.Node) sign.Round {
			return NewRoundCosiStamper(node)
		})
}

func NewRoundCosiStamper(node *sign.Node) sign.Round {
	dbg.Lvlf3("Making new roundcosistamper %+v", node)
	round := &RoundCosiStamper{}
	round.RoundStamper = NewRoundStamper(node)
	round.RoundCosi = sign.NewRoundCosi(node)
	round.RoundStruct = sign.NewRoundStruct(node)
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
	round.Mux.Lock()
	// get data from s once to avoid refetching from structure
	round.RoundStamper.QueueSet(round.Queue)
	round.Mux.Unlock()

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
