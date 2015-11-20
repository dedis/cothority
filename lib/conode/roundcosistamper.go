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
	rt.RoundStamper.Announcement(round, in, out)
	rt.RoundCosi.Announcement(round, in, out)
	return nil
}

func (rt *RoundCosiStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	rt.RoundStamper.Commitment(in, out)
	rt.RoundCosi.Commitment(in, out)
	return nil
}

func (rt *RoundCosiStamper) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	rt.RoundStamper.Challenge(in, out)
	rt.RoundCosi.Challenge(in, out)
	return nil
}

func (rt *RoundCosiStamper) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	rt.RoundStamper.Response(in, out)
	rt.RoundCosi.Response(in, out)
	return nil
}

func (rt *RoundCosiStamper) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	rt.RoundStamper.SignatureBroadcast(in, out)
	rt.RoundCosi.SignatureBroadcast(in, out)
	return nil
}
