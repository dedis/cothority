package conode
import "github.com/dedis/cothority/lib/sign"

/*
Implements the basic Collective Signature using Schnorr signatures.
 */

const RoundCosiType = "cosi"

type RoundCosi struct {
}

func init() {
	sign.RegisterRoundFactory(RoundCosiType,
		func(s *sign.Node) sign.Round {
			return NewRoundCosi(s)
		})
}

func NewRoundCosi(node *sign.Node) *RoundCosi{
	round := &RoundCosi{}
	return round
}

func (round *RoundCosi) Announcement(RoundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}

func (round *RoundCosi) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	return nil
}

func (round *RoundCosi) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}

func (round *RoundCosi) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	return nil
}

func (round *RoundCosi) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}
