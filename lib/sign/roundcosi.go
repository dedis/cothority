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
	rt := &RoundCosi{}
	return rt
}

func (rt *RoundCosi) Announcement(round int, in *SigningMessage,
out []*SigningMessage) error {
	return nil
}

func (rt *RoundCosi) Commitment(in []*SigningMessage, out *SigningMessage) error {
	return nil
}

func (rt *RoundCosi) Challenge(in *SigningMessage, out []*SigningMessage) error {
	return nil
}

func (rt *RoundCosi) Response(in []*SigningMessage, out *SigningMessage) error {
	return nil
}

func (rt *RoundCosi) SignatureBroadcast(in *SigningMessage, out []*SigningMessage) error {
	return nil
}
