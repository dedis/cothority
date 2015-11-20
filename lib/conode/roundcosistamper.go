package conode
import "github.com/dedis/cothority/lib/sign"

/*
Implements a test-round which uses RoundCosi and RoundStamp
 */

const RoundCosiStamperType = "test"

type RoundCosiStamper struct {
	*RoundStamper
	*sign.RoundCosi
}

func RegisterRoundCosiStamper(p *Peer) {
	sign.RegisterRoundFactory(RoundCosiStamperType,
		func(s *sign.Node) sign.Round {
			return NewRoundCosiStamper(p)
		})
}

func NewRoundCosiStamper(peer *Peer) *RoundCosiStamper{
	rt := &RoundCosiStamper{}
	rt.RoundStamper = NewRoundStamper(peer)
	rt.RoundCosi = sign.NewRoundCosi(peer.Node)
	return rt
}

func (rt *RoundCosiStamper) Announcement(round int, in *sign.SigningMessage,
out []*sign.SigningMessage) error {
	rt.RoundCosi.Announcement(round, in, out)
	return rt.RoundStamper.Announcement(round, in, out)
}

func (rt *RoundCosiStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	rt.RoundCosi.Commitment(in, out)
	return rt.RoundStamper.Commitment(in, out)
}

func (rt *RoundCosiStamper) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	rt.RoundCosi.Challenge(in, out)
	return rt.RoundStamper.Challenge(in, out)
}

func (rt *RoundCosiStamper) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	rt.RoundCosi.Response(in, out)
	return rt.RoundStamper.Response(in, out)
}

func (rt *RoundCosiStamper) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	rt.RoundCosi.SignatureBroadcast(in, out)
	return rt.RoundStamper.SignatureBroadcast(in, out)
}
