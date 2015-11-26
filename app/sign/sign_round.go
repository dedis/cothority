package main
import (
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/dbg"
)

const RoundMeasureType = "measure"

type RoundMeasure struct {
	measure *monitor.Measure
	*sign.RoundCosi
}

func init() {
	sign.RegisterRoundFactory(RoundMeasureType,
		func(s *sign.Node) sign.Round {
			return NewRoundMeasure(s)
		})
}

func NewRoundMeasure(node *sign.Node) *RoundMeasure {
	dbg.Lvlf3("Making new roundmeasure %+v", node)
	round := &RoundMeasure{}
	round.RoundCosi = sign.NewRoundCosi(node)
	round.Type = RoundMeasureType
	return round
}

func (round *RoundMeasure) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	if round.IsRoot {
		round.measure = monitor.NewMeasure("round")
	}
	return round.RoundCosi.Announcement(viewNbr, roundNbr, in, out)
}

func (round *RoundMeasure)Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	err := round.RoundCosi.Response(in, out)
	if round.IsRoot {
		round.measure.Measure()
	}
	return err
}