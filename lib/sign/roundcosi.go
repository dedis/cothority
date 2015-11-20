package sign

/*
Implements the basic Collective Signature using Schnorr signatures.
 */

const RoundCosiType = "cosi"

type RoundCosi struct {
}

func init() {
	RegisterRoundFactory(RoundCosiType,
		func(s *Node) Round {
			return NewRoundCosi(s)
		})
}

func NewRoundCosi(node *Node) *RoundCosi{
	round := &RoundCosi{}
	return round
}

func (round *RoundCosi) Announcement(RoundNbr int, in *SigningMessage, out []*SigningMessage) error {
	return nil
}

func (round *RoundCosi) Commitment(in []*SigningMessage, out *SigningMessage) error {
	return nil
}

func (round *RoundCosi) Challenge(in *SigningMessage, out []*SigningMessage) error {
	return nil
}

func (round *RoundCosi) Response(in []*SigningMessage, out *SigningMessage) error {
	return nil
}

func (round *RoundCosi) SignatureBroadcast(in *SigningMessage, out []*SigningMessage) error {
	return nil
}
