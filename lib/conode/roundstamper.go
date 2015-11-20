package conode
import "github.com/dedis/cothority/lib/sign"

/*
Implements the basic Collective Signature using Schnorr signatures.
 */

const RoundStamperType = "stamper"

type RoundStamper struct {
	peer *Peer
}

func RegisterRoundStamper(p *Peer) {
	sign.RegisterRoundFactory(RoundStamperType,
		func(s *sign.Node) sign.Round {
			return NewRoundStamper(p)
		})
}

func NewRoundStamper(peer *Peer) *RoundStamper {
	round := &RoundStamper{}
	return round
}

func (round *RoundStamper) Announcement(RoundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	in.Am.RoundType = RoundCosiStamperType
	return nil
}

func (round *RoundStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {

	return nil
}

func (round *RoundStamper) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}

func (round *RoundStamper) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	return nil
}

func (round *RoundStamper) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}
